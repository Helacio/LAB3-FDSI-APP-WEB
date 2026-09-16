package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// handleStatusStream entrega el progreso de un run por Server-Sent Events.
// Si runId esta vacio, transmite los eventos de todos los runs.
func (s *server) handleStatusStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming no soportado"})
		return
	}

	// El servidor tiene WriteTimeout; para SSE lo desactivamos en esta conexion.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // evita buffering de proxies (nginx)
	w.WriteHeader(http.StatusOK)

	ctx := r.Context()
	runFilter := r.URL.Query().Get("runId")

	events, cancel, err := s.broker.SubscribeEvents(ctx)
	if err != nil {
		log.Printf("no se pudo suscribir al broker para SSE: %v", err)
		fmt.Fprintf(w, "event: stream.error\ndata: {\"error\":\"broker no disponible\"}\n\n")
		flusher.Flush()
		return
	}
	defer cancel()

	fmt.Fprint(w, ": stream MuvAutomation abierto\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case ev, open := <-events:
			if !open {
				return
			}
			if runFilter != "" && ev.RunID != runFilter {
				continue
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\n", ev.Type)
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
