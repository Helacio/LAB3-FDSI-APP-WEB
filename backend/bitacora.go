package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func payloadSesion(d session) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", d.Dispositivo, d.Comando, d.Operador, d.Timestamp, d.Resultado)
}

func (s *server) sellar(prevHash string, d session) (string, string) {
	h := sha256.Sum256([]byte(prevHash + payloadSesion(d)))
	hashHex := hex.EncodeToString(h[:])
	mac := hmac.New(sha256.New, s.bitacoraKey)
	mac.Write([]byte(hashHex))
	return hashHex, hex.EncodeToString(mac.Sum(nil))
}

func (s *server) ultimaSesion(ctx context.Context) (*session, error) {
	cur, err := s.db.Collection("sessions").Find(ctx, bson.D{},
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(1))
	if err != nil {
		return nil, err
	}
	defer cur.Close(context.Background())

	var out []session
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}

func (s *server) handleVerifyBitacora(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	cur, err := s.db.Collection("sessions").Find(ctx, bson.D{},
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer la bitacora"})
		return
	}
	defer cur.Close(context.Background())

	var docs []session
	if err := cur.All(ctx, &docs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer la bitacora"})
		return
	}

	prev := ""
	legadas := 0
	primerRompimiento := ""
	ok := true
	for _, d := range docs {
		if d.Hash == "" {
			legadas++
			prev = ""
			continue
		}
		hashEsp, hmacEsp := s.sellar(prev, d)
		if !hmac.Equal([]byte(hashEsp), []byte(d.Hash)) || !hmac.Equal([]byte(hmacEsp), []byte(d.HMAC)) {
			ok = false
			primerRompimiento = d.Timestamp
			break
		}
		prev = d.Hash
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                ok,
		"total":             len(docs),
		"legadas":           legadas,
		"primerRompimiento": primerRompimiento,
	})
}
