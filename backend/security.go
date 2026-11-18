package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Tamaño maximo aceptado para cuerpos JSON (A04/A05: evita DoS por payloads enormes).
const maxBodyBytes = 16 << 10 // 16 KiB

// minSecretBytes es la longitud minima exigida a JWT_SECRET y BITACORA_KEY.
const minSecretBytes = 32

// decodeJSON limita el tamaño del cuerpo, rechaza campos desconocidos y basura al final.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("el cuerpo debe contener un unico objeto JSON")
	}
	return nil
}

// securityHeaders evita que respuestas con datos autenticados queden en caches
// del navegador o de proxies (A04/A05, ZAP 10015/10049). El resto de cabeceras
// de seguridad (CSP, XFO, nosniff, COOP/CORP) las agrega nginx para no duplicarlas.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("Pragma", "no-cache")
		next.ServeHTTP(w, r)
	})
}

// secretoObligatorio lee un secreto del entorno y aborta si falta o es debil.
// No existe valor por defecto: un secreto embebido en el codigo es publico (A02/A07).
func secretoObligatorio(key string) ([]byte, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if len(v) < minSecretBytes {
		return nil, fmt.Errorf("%s no definido o con menos de %d caracteres", key, minSecretBytes)
	}
	return []byte(v), nil
}

// redactarURL oculta usuario/clave de una URI de conexion antes de registrarla (A09).
func redactarURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<uri invalida>"
	}
	if u.User != nil {
		u.User = url.User("***")
	}
	return u.Redacted()
}

// auditar registra eventos de seguridad sin datos sensibles (A09).
func auditar(evento string, r *http.Request, usuario, detalle string) {
	log.Printf("audit evento=%s ip=%s usuario=%q detalle=%q", evento, ipCliente(r), usuario, detalle)
}
