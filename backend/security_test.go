package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestIpClienteIgnoraXForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.RemoteAddr = "203.0.113.7:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.Header.Set("X-Real-IP", "5.6.7.8")
	if got := ipCliente(r); got != "203.0.113.7" {
		t.Fatalf("cliente remoto no debe poder falsificar su IP: got %s", got)
	}
}

func TestIpClienteConfiaEnProxyLocal(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.RemoteAddr = "127.0.0.1:5555"
	r.Header.Set("X-Real-IP", "198.51.100.9")
	if got := ipCliente(r); got != "198.51.100.9" {
		t.Fatalf("debe usar X-Real-IP puesto por nginx: got %s", got)
	}
}

func TestLimiter(t *testing.T) {
	l := newLoginLimiter(3)
	for i := 0; i < 3; i++ {
		if !l.permitido("x") {
			t.Fatalf("intento %d deberia permitirse", i)
		}
		l.registrar("x")
	}
	if l.permitido("x") {
		t.Fatal("el 4to intento debe bloquearse")
	}
}

func TestPasoTOTPAntiReplay(t *testing.T) {
	secreto := generarSecreto()
	codigo, err := totp.GenerateCodeCustom(secreto, time.Now().UTC(), totp.ValidateOpts{
		Period: totpPeriodo, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatal(err)
	}
	paso, ok := pasoTOTP(codigo, secreto, 0)
	if !ok {
		t.Fatal("codigo valido rechazado")
	}
	if _, ok := pasoTOTP(codigo, secreto, paso); ok {
		t.Fatal("un codigo ya consumido no debe aceptarse (replay)")
	}
	if _, ok := pasoTOTP("000000", "", 0); ok {
		t.Fatal("secreto vacio debe rechazarse")
	}
}

func TestDecodeJSONLimitesYCampos(t *testing.T) {
	var in struct {
		A string `json:"a"`
	}
	casos := map[string]bool{
		`{"a":"ok"}`:              true,
		`{"a":"ok","admin":true}`: false, // campo desconocido
		`{"a":"ok"}{"a":"x"}`:     false, // basura al final
		`{"a":"` + strings.Repeat("x", maxBodyBytes) + `"}`: false, // demasiado grande
	}
	for body, want := range casos {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		if got := decodeJSON(w, r, &in) == nil; got != want {
			t.Errorf("body len=%d: got ok=%v want %v", len(body), got, want)
		}
	}
}

func TestRedactarURL(t *testing.T) {
	got := redactarURL("amqp://guest:guest@127.0.0.1:5672/")
	if strings.Contains(got, "guest") {
		t.Fatalf("credenciales filtradas en log: %s", got)
	}
}

func TestStreamAuthRechazaTokenDeAcceso(t *testing.T) {
	s := &server{jwtSecret: []byte(strings.Repeat("k", 32))}
	acceso, _ := s.emitirToken("u", rolLector, "acceso", time.Minute)
	sse, _ := s.emitirToken("u", rolLector, "sse", time.Minute)
	h := s.requireStreamAuth(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	for tok, want := range map[string]int{acceso: http.StatusUnauthorized, sse: http.StatusOK, "": http.StatusUnauthorized} {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodGet, "/api/status/stream?ticket="+tok, nil))
		if w.Code != want {
			t.Errorf("got %d want %d", w.Code, want)
		}
	}
	// El token de acceso tampoco se acepta por query en rutas normales.
	w := httptest.NewRecorder()
	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {})(w, httptest.NewRequest(http.MethodGet, "/api/kpis?token="+acceso, nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("token en query aceptado: %d", w.Code)
	}
}
