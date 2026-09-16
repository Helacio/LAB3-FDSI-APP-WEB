package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	mathrand "math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type statusRun struct {
	RunID        string `json:"runId" bson:"runId"`
	Operador     string `json:"operador" bson:"operador"`
	Total        int    `json:"total" bson:"total"`
	Completados  int    `json:"completados" bson:"completados"`
	Estado       string `json:"estado" bson:"estado"`
	IniciadoEn   string `json:"iniciadoEn" bson:"iniciadoEn"`
	FinalizadoEn string `json:"finalizadoEn,omitempty" bson:"finalizadoEn,omitempty"`
}

type statusCheck struct {
	RunID      string `json:"runId" bson:"runId"`
	Hostname   string `json:"hostname" bson:"hostname"`
	Gestion    string `json:"gestion" bson:"gestion"`
	Estado     string `json:"estado" bson:"estado"`
	LatenciaMs int    `json:"latenciaMs" bson:"latenciaMs"`
	Detalle    string `json:"detalle" bson:"detalle"`
	Timestamp  string `json:"timestamp" bson:"timestamp"`
}

type statusManager struct {
	db      *mongo.Database
	broker  Broker
	workers int
}

func newStatusManager(db *mongo.Database, broker Broker, workers int) *statusManager {
	if workers < 1 {
		workers = 1
	}
	return &statusManager{db: db, broker: broker, workers: workers}
}

// StartRun crea un run, publica el evento inicial y encola un job por dispositivo.
func (m *statusManager) StartRun(ctx context.Context, operador string) (string, int, error) {
	cur, err := m.db.Collection("devices").Find(ctx, bson.D{})
	if err != nil {
		return "", 0, err
	}
	var devices []device
	if err := cur.All(ctx, &devices); err != nil {
		cur.Close(context.Background())
		return "", 0, err
	}
	cur.Close(context.Background())
	if len(devices) == 0 {
		return "", 0, errors.New("no hay dispositivos para revisar")
	}

	runID := newID()
	now := time.Now().UTC().Format(time.RFC3339)
	run := statusRun{
		RunID:      runID,
		Operador:   orDefault(operador, "scheduler"),
		Total:      len(devices),
		Estado:     "en_progreso",
		IniciadoEn: now,
	}
	if _, err := m.db.Collection("status_runs").InsertOne(ctx, run); err != nil {
		return "", 0, err
	}

	if err := m.broker.PublishEvent(ctx, statusEvent{
		Type:      "run.started",
		RunID:     runID,
		Operador:  run.Operador,
		Total:     len(devices),
		Timestamp: now,
	}); err != nil {
		log.Printf("no se pudo publicar run.started: %v", err)
	}

	for _, d := range devices {
		job := statusJob{
			RunID:       runID,
			Hostname:    d.Hostname,
			Gestion:     d.Gestion,
			Operador:    run.Operador,
			RequestedAt: now,
		}
		if err := m.broker.PublishJob(ctx, job); err != nil {
			return runID, len(devices), err
		}
	}
	return runID, len(devices), nil
}

// RunWorkers arranca el pool de workers que consume la cola de trabajos.
func (m *statusManager) RunWorkers(ctx context.Context) error {
	log.Printf("worker pool de status listo (%d workers sobre %q)", m.workers, jobsQueueName)
	return m.broker.ConsumeJobs(ctx, m.workers, func(job statusJob) {
		m.handleJob(ctx, job)
	})
}

func (m *statusManager) handleJob(ctx context.Context, job statusJob) {
	// Chequeo simulado: latencia artificial acotada (los equipos son ficticios).
	delay := time.Duration(200+mathrand.IntN(700)) * time.Millisecond
	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	estado, latencia, detalle := simulateCheck()
	now := time.Now().UTC().Format(time.RFC3339)
	check := statusCheck{
		RunID:      job.RunID,
		Hostname:   job.Hostname,
		Gestion:    job.Gestion,
		Estado:     estado,
		LatenciaMs: latencia,
		Detalle:    detalle,
		Timestamp:  now,
	}
	if _, err := m.db.Collection("status_checks").InsertOne(ctx, check); err != nil {
		log.Printf("no se pudo guardar el check de %s: %v", job.Hostname, err)
	}
	_, _ = m.db.Collection("devices").UpdateOne(ctx,
		bson.D{{Key: "hostname", Value: job.Hostname}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: estado}}}},
	)

	var run statusRun
	err := m.db.Collection("status_runs").FindOneAndUpdate(ctx,
		bson.D{{Key: "runId", Value: job.RunID}},
		bson.D{{Key: "$inc", Value: bson.D{{Key: "completados", Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&run)
	if err != nil {
		log.Printf("no se pudo actualizar el progreso del run %s: %v", job.RunID, err)
		return
	}

	_ = m.broker.PublishEvent(ctx, statusEvent{
		Type:        "check.result",
		RunID:       job.RunID,
		Hostname:    job.Hostname,
		Estado:      estado,
		LatenciaMs:  latencia,
		Detalle:     detalle,
		Completados: run.Completados,
		Total:       run.Total,
		Timestamp:   now,
	})

	if run.Completados == run.Total {
		m.finalizeRun(ctx, run)
	}
}

func (m *statusManager) finalizeRun(ctx context.Context, run statusRun) {
	resumen := map[string]int{}
	cur, err := m.db.Collection("status_checks").Find(ctx, bson.D{{Key: "runId", Value: run.RunID}})
	if err == nil {
		for cur.Next(ctx) {
			var c statusCheck
			if cur.Decode(&c) == nil {
				resumen[c.Estado]++
			}
		}
		cur.Close(context.Background())
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = m.db.Collection("status_runs").UpdateOne(ctx,
		bson.D{{Key: "runId", Value: run.RunID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "estado", Value: "finalizado"},
			{Key: "finalizadoEn", Value: now},
		}}},
	)
	_ = m.broker.PublishEvent(ctx, statusEvent{
		Type:        "run.finished",
		RunID:       run.RunID,
		Completados: run.Completados,
		Total:       run.Total,
		Resumen:     resumen,
		Timestamp:   now,
	})
}

// RunScheduler dispara runs periodicos mientras no haya uno en curso.
func (m *statusManager) RunScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		log.Println("scheduler de status desactivado (STATUS_INTERVAL=0)")
		return
	}
	log.Printf("scheduler de status activo cada %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if m.hasActiveRun(ctx) {
				log.Println("hay un run de status en curso; se omite el periodico")
				continue
			}
			runID, total, err := m.StartRun(ctx, "scheduler")
			if err != nil {
				log.Printf("error en run periodico: %v", err)
				continue
			}
			log.Printf("run periodico %s encolado (%d dispositivos)", runID, total)
		}
	}
}

func (m *statusManager) hasActiveRun(ctx context.Context) bool {
	n, err := m.db.Collection("status_runs").CountDocuments(ctx, bson.D{{Key: "estado", Value: "en_progreso"}})
	return err == nil && n > 0
}

// simulateCheck devuelve un estado ponderado. Los equipos del lab son ficticios,
// por lo que no se hace un probe de red real.
func simulateCheck() (estado string, latencia int, detalle string) {
	switch n := mathrand.IntN(100); {
	case n < 78:
		return "en línea", 40 + mathrand.IntN(220), "SNMP y SSH responden; contadores sin errores"
	case n < 90:
		return "mantenimiento", 60 + mathrand.IntN(200), "ventana de mantenimiento programada"
	default:
		return "fuera de gestión", 0, "sin respuesta al probe de gestión (timeout)"
	}
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf)
}

// --- Handlers HTTP ---

func (s *server) handleStartStatusCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	var in struct {
		Operador string `json:"operador"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)

	runID, total, err := s.status.StartRun(ctx, in.Operador)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo iniciar la revision: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"runId": runID, "total": total})
}

func (s *server) handleStatusRuns(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	cur, err := s.db.Collection("status_runs").Find(ctx, bson.D{},
		options.Find().SetSort(bson.D{{Key: "iniciadoEn", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer los runs"})
		return
	}
	defer cur.Close(context.Background())

	var runs []statusRun
	if err := cur.All(ctx, &runs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer los runs"})
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *server) handleStatusRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	runID := r.PathValue("runId")
	var run statusRun
	if err := s.db.Collection("status_runs").FindOne(ctx, bson.D{{Key: "runId", Value: runID}}).Decode(&run); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run no encontrado"})
		return
	}

	cur, err := s.db.Collection("status_checks").Find(ctx,
		bson.D{{Key: "runId", Value: runID}},
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}}))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudieron leer los checks"})
		return
	}
	defer cur.Close(context.Background())

	var checks []statusCheck
	if err := cur.All(ctx, &checks); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudieron leer los checks"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "checks": checks})
}

func (s *server) handleStatusLatest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	cur, err := s.db.Collection("devices").Find(ctx, bson.D{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer el inventario"})
		return
	}
	defer cur.Close(context.Background())

	var devices []device
	if err := cur.All(ctx, &devices); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer el inventario"})
		return
	}

	out := make([]statusCheck, 0, len(devices))
	for _, d := range devices {
		var c statusCheck
		err := s.db.Collection("status_checks").FindOne(ctx,
			bson.D{{Key: "hostname", Value: d.Hostname}},
			options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: -1}}),
		).Decode(&c)
		if err == nil {
			out = append(out, c)
		}
	}
	writeJSON(w, http.StatusOK, out)
}
