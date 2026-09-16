package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

const (
	defaultPort     = "8080"
	defaultMongoURI = "mongodb://127.0.0.1:27017"
	defaultMongoDB  = "muvautomation"

	envEntorno     = "LAB"
	envPropietario = "Blue Team"
	envAviso       = "Entorno de laboratorio. Todos los datos son ficticios."
)

type device struct {
	Hostname string `json:"hostname" bson:"hostname"`
	Tipo     string `json:"tipo" bson:"tipo"`
	Sitio    string `json:"sitio" bson:"sitio"`
	Rol      string `json:"rol" bson:"rol"`
	Gestion  string `json:"gestion" bson:"gestion"`
	Estado   string `json:"estado" bson:"estado"`
}

type session struct {
	Dispositivo string `json:"dispositivo" bson:"dispositivo"`
	Comando     string `json:"comando" bson:"comando"`
	Operador    string `json:"operador" bson:"operador"`
	Timestamp   string `json:"timestamp" bson:"timestamp"`
	Resultado   string `json:"resultado" bson:"resultado"`
}

type server struct {
	db *mongo.Database
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("error serializando respuesta: %v", err)
	}
}

func (s *server) ctx(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 5*time.Second)
}

func (s *server) settingsMap(ctx context.Context) (map[string]string, error) {
	cur, err := s.db.Collection("settings").Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(context.Background())

	out := map[string]string{}
	for cur.Next(ctx) {
		var doc struct {
			Clave string `bson:"clave"`
			Valor string `bson:"valor"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		out[doc.Clave] = doc.Valor
	}
	return out, cur.Err()
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()
	pingCtx, pingCancel := s.ctx(ctx)
	defer pingCancel()
	if err := s.db.Client().Ping(pingCtx, readpref.Primary()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "mongodb": "down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "mongodb": "up"})
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	cfg, err := s.settingsMap(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer la configuracion"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"entorno":     orDefault(cfg["entorno"], envEntorno),
		"propietario": orDefault(cfg["propietario"], envPropietario),
		"aviso":       orDefault(cfg["aviso"], envAviso),
	})
}

func (s *server) handleKpis(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	devices, err := s.db.Collection("devices").CountDocuments(ctx, bson.D{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo contar dispositivos"})
		return
	}
	commands, err := s.db.Collection("commands").CountDocuments(ctx, bson.D{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo contar comandos"})
		return
	}

	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	sessionsToday, err := s.db.Collection("sessions").CountDocuments(ctx, bson.D{
		{Key: "timestamp", Value: bson.M{
			"$gte": startOfDay.Format(time.RFC3339),
			"$lt":  startOfDay.Add(24 * time.Hour).Format(time.RFC3339),
		}},
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudieron contar sesiones"})
		return
	}

	cfg, _ := s.settingsMap(ctx)
	writeJSON(w, http.StatusOK, map[string]any{
		"dispositivosGestionados": devices,
		"comandosPermitidos":      commands,
		"sesionesHoy":             sessionsToday,
		"propietario":             orDefault(cfg["propietario"], envPropietario),
	})
}

func (s *server) handleInventory(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, devices)
}

func (s *server) handleCommands(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	cur, err := s.db.Collection("commands").Find(ctx, bson.D{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer el catalogo"})
		return
	}
	defer cur.Close(context.Background())

	var out []string
	for cur.Next(ctx) {
		var doc struct {
			Comando string `bson:"comando"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		out = append(out, doc.Comando)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleInventoryTxt(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	var devices []device
	cur, err := s.db.Collection("devices").Find(ctx, bson.D{})
	if err == nil {
		cur.All(ctx, &devices)
		cur.Close(context.Background())
	}

	var lines []string
	lines = append(lines,
		"MuvAutomation - Portal de Ejecucion Remota Segura",
		"Inventario publico de demostracion (datos ficticios)",
		"Entorno: LAB | Propietario: Blue Team",
		"",
		"hostname     tipo        sitio    rol             gestion       estado",
		"-----------  ----------  -------  --------------  ------------  ------------------",
	)
	for _, d := range devices {
		lines = append(lines, fmt.Sprintf(
			"%-12v %-10v %-7v %-14v %-12v %-18v",
			d.Hostname, d.Tipo, d.Sitio, d.Rol, d.Gestion, d.Estado,
		))
	}
	lines = append(lines,
		"",
		"Nota: rangos IP de documentacion segun RFC 5737. Sin credenciales ni datos reales.",
		"",
	)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strings.Join(lines, "\n")))
}

func (s *server) handleSessions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	cur, err := s.db.Collection("sessions").Find(
		ctx,
		bson.D{},
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer la bitacora"})
		return
	}
	defer cur.Close(context.Background())

	var sessions []session
	if err := cur.All(ctx, &sessions); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer la bitacora"})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	var in struct {
		Dispositivo string `json:"dispositivo"`
		Comando     string `json:"comando"`
		Operador    string `json:"operador"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalido"})
		return
	}
	in.Dispositivo = strings.TrimSpace(in.Dispositivo)
	in.Comando = strings.TrimSpace(in.Comando)
	in.Operador = strings.TrimSpace(in.Operador)
	if in.Dispositivo == "" || in.Comando == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dispositivo y comando son obligatorios"})
		return
	}

	doc := session{
		Dispositivo: in.Dispositivo,
		Comando:     in.Comando,
		Operador:    orDefault(in.Operador, "Anonimo"),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Resultado:   "OK",
	}
	if _, err := s.db.Collection("sessions").InsertOne(ctx, doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo registrar la sesion"})
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func orDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func seedData(ctx context.Context, db *mongo.Database) error {
	seed := func(name string, docs []any, upsertKey string) error {
		coll := db.Collection(name)
		count, err := coll.CountDocuments(ctx, bson.D{})
		if err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		keys := []string{upsertKey}
		for _, raw := range docs {
			doc := raw.(bson.D)
			var filter bson.D
			for _, k := range keys {
				for _, e := range doc {
					if e.Key == k {
						filter = append(filter, bson.E{Key: k, Value: e.Value})
					}
				}
			}
			if _, err := coll.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: doc}}, options.UpdateOne().SetUpsert(true)); err != nil {
				return err
			}
		}
		return nil
	}

	devices := []any{
		bson.D{{Key: "hostname", Value: "RTR-LAB-01"}, {Key: "tipo", Value: "Router"}, {Key: "sitio", Value: "Sitio A"}, {Key: "rol", Value: "Borde / WAN"}, {Key: "gestion", Value: "192.0.2.11"}, {Key: "estado", Value: "en línea"}},
		bson.D{{Key: "hostname", Value: "RTR-LAB-02"}, {Key: "tipo", Value: "Router"}, {Key: "sitio", Value: "Sitio B"}, {Key: "rol", Value: "Borde / WAN"}, {Key: "gestion", Value: "192.0.2.12"}, {Key: "estado", Value: "en línea"}},
		bson.D{{Key: "hostname", Value: "SW-LAB-01"}, {Key: "tipo", Value: "Switch L3"}, {Key: "sitio", Value: "Sitio A"}, {Key: "rol", Value: "Distribución"}, {Key: "gestion", Value: "192.0.2.21"}, {Key: "estado", Value: "en línea"}},
		bson.D{{Key: "hostname", Value: "SW-LAB-02"}, {Key: "tipo", Value: "Switch L2"}, {Key: "sitio", Value: "Sitio A"}, {Key: "rol", Value: "Acceso"}, {Key: "gestion", Value: "192.0.2.22"}, {Key: "estado", Value: "mantenimiento"}},
		bson.D{{Key: "hostname", Value: "FW-LAB-01"}, {Key: "tipo", Value: "Firewall"}, {Key: "sitio", Value: "Sitio A"}, {Key: "rol", Value: "Perímetro"}, {Key: "gestion", Value: "192.0.2.31"}, {Key: "estado", Value: "fuera de gestión"}},
	}
	if err := seed("devices", devices, "hostname"); err != nil {
		return err
	}

	commands := []any{
		bson.D{{Key: "comando", Value: "show version"}},
		bson.D{{Key: "comando", Value: "show inventory"}},
		bson.D{{Key: "comando", Value: "show interfaces status"}},
		bson.D{{Key: "comando", Value: "show ip interface brief"}},
		bson.D{{Key: "comando", Value: "show cdp neighbors"}},
		bson.D{{Key: "comando", Value: "show vlan brief"}},
		bson.D{{Key: "comando", Value: "show running-config | section snmp"}},
		bson.D{{Key: "comando", Value: "show logging last 50"}},
	}
	if err := seed("commands", commands, "comando"); err != nil {
		return err
	}

	settings := []any{
		bson.D{{Key: "clave", Value: "entorno"}, {Key: "valor", Value: envEntorno}},
		bson.D{{Key: "clave", Value: "propietario"}, {Key: "valor", Value: envPropietario}},
		bson.D{{Key: "clave", Value: "aviso"}, {Key: "valor", Value: envAviso}},
	}
	return seed("settings", settings, "clave")
}

func main() {
	port := ":" + envOr("PORT", defaultPort)
	uri := envOr("MONGO_URI", defaultMongoURI)
	dbName := envOr("MONGO_DB", defaultMongoDB)

	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		log.Fatalf("no se pudo configurar el cliente de MongoDB: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatalf("MongoDB no responde en %s: %v", uri, err)
	}
	log.Printf("conectado a MongoDB en %s", uri)

	db := client.Database(dbName)
	if err := seedData(ctx, db); err != nil {
		log.Fatalf("no se pudo sembrar la base de datos: %v", err)
	}

	s := &server{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/kpis", s.handleKpis)
	mux.HandleFunc("GET /api/inventory", s.handleInventory)
	mux.HandleFunc("GET /api/inventory.txt", s.handleInventoryTxt)
	mux.HandleFunc("GET /api/commands", s.handleCommands)
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /api/healthz", s.handleHealth)

	httpServer := &http.Server{
		Addr:         port,
		Handler:      cors(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("backend MuvAutomation escuchando en %s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("error del servidor HTTP: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("deteniendo servidor...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = client.Disconnect(shutdownCtx)
	log.Println("servidor detenido")
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
