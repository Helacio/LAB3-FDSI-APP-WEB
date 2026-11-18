package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

const (
	rolLector    = "lector"
	rolCambiador = "cambiador"
)

const (
	accessTokenTTL = 15 * time.Minute
	ticketTTL      = 2 * time.Minute
	streamTTL      = 1 * time.Minute
)

const (
	maxLoginIntentos  = 10
	maxVerifyIntentos = 5
	ventanaLogin      = 5 * time.Minute
	totpPeriodo       = 30
)

type user struct {
	Usuario    string `bson:"usuario" json:"usuario"`
	Hash       string `bson:"hash" json:"-"`
	Rol        string `bson:"rol" json:"rol"`
	TOTPSecret string `bson:"totpSecret,omitempty" json:"-"`
	Pendiente  string `bson:"pendiente,omitempty" json:"-"`
	MfaActivo  bool   `bson:"mfaActivo" json:"mfaActivo"`
	UltimoPaso int64  `bson:"ultimoPasoTotp,omitempty" json:"-"`
	CreadoEn   string `bson:"creadoEn" json:"creadoEn"`
}

type ctxKey string

const (
	ctxUsuarioKey ctxKey = "usuario"
	ctxRolKey     ctxKey = "rol"
)

type authClaims struct {
	Rol  string `json:"rol,omitempty"`
	Tipo string `json:"tipo,omitempty"` // "acceso" | "mfa" | "enroll" | "sse"
	jwt.RegisteredClaims
}

type loginLimiter struct {
	mu       sync.Mutex
	max      int
	attempts map[string][]time.Time
}

func newLoginLimiter(max int) *loginLimiter {
	return &loginLimiter{max: max, attempts: map[string][]time.Time{}}
}

func (l *loginLimiter) permitido(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	corte := time.Now().Add(-ventanaLogin)
	var vivos []time.Time
	for _, t := range l.attempts[ip] {
		if t.After(corte) {
			vivos = append(vivos, t)
		}
	}
	if len(vivos) == 0 {
		delete(l.attempts, ip) // evita crecimiento ilimitado del mapa
		return true
	}
	l.attempts[ip] = vivos
	return len(vivos) < l.max
}

func (l *loginLimiter) registrar(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts[ip] = append(l.attempts[ip], time.Now())
}

func (l *loginLimiter) limpiar(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

func generarSecreto() string {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		panic("sin entropia para generar secreto TOTP")
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
}

func otpauthURL(usuario, secreto string) string {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MuvAutomation",
		AccountName: usuario,
		Secret:      []byte(secreto),
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return ""
	}
	return key.URL()
}

func (s *server) emitirToken(usuario, rol, tipo string, ttl time.Duration) (string, error) {
	ahora := time.Now().UTC()
	c := authClaims{
		Rol:  rol,
		Tipo: tipo,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   usuario,
			IssuedAt:  jwt.NewNumericDate(ahora),
			ExpiresAt: jwt.NewNumericDate(ahora.Add(ttl)),
			Issuer:    "muvautomation",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.jwtSecret)
}

func (s *server) validarToken(tokenStr string) (*authClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &authClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("metodo de firma inesperado")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := token.Claims.(*authClaims)
	if !ok || !token.Valid {
		return nil, errors.New("token invalido")
	}
	return c, nil
}

// ipCliente devuelve la IP real del cliente. X-Forwarded-For lo controla el cliente
// (nginx solo lo extiende), por lo que NO se usa: un atacante podria rotarlo para
// evadir el rate limit. Solo se confia en X-Real-IP cuando la conexion viene del
// proxy local (nginx lo sobrescribe con $remote_addr).
func ipCliente(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if real := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); real != nil {
			return real.String()
		}
	}
	return host
}

func bearerDe(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func credenciales(ctx context.Context) (string, string) {
	usuario, _ := ctx.Value(ctxUsuarioKey).(string)
	rol, _ := ctx.Value(ctxRolKey).(string)
	return usuario, rol
}

func usuarioDe(ctx context.Context) string {
	usuario, _ := credenciales(ctx)
	return usuario
}

func (s *server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Solo cabecera Authorization: un token en la URL queda en logs/historial (A01/A09).
		tokenStr := bearerDe(r)
		if tokenStr == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "autenticacion requerida"})
			return
		}
		c, err := s.validarToken(tokenStr)
		if err != nil || c.Tipo != "acceso" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "token invalido o vencido"})
			return
		}
		ctx := context.WithValue(r.Context(), ctxUsuarioKey, c.Subject)
		ctx = context.WithValue(ctx, ctxRolKey, c.Rol)
		next(w, r.WithContext(ctx))
	}
}

// requireStreamAuth autentica el SSE. EventSource no permite cabeceras, asi que se
// acepta en la query un ticket de un solo proposito (tipo "sse", 1 minuto) emitido
// por POST /api/auth/stream-ticket; el token de acceso nunca viaja en la URL.
func (s *server) requireStreamAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := s.validarToken(r.URL.Query().Get("ticket"))
		if err != nil || c.Tipo != "sse" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "ticket de stream invalido o vencido"})
			return
		}
		ctx := context.WithValue(r.Context(), ctxUsuarioKey, c.Subject)
		ctx = context.WithValue(ctx, ctxRolKey, c.Rol)
		next(w, r.WithContext(ctx))
	}
}

func (s *server) handleStreamTicket(w http.ResponseWriter, r *http.Request) {
	usuario, rol := credenciales(r.Context())
	ticket, err := s.emitirToken(usuario, rol, "sse", streamTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo emitir el ticket"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ticket": ticket})
}

func (s *server) requireRole(rol string, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if usuario, rolActual := credenciales(r.Context()); rolActual != rol {
			auditar("acceso_denegado", r, usuario, r.Method+" "+r.URL.Path)
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "rol insuficiente: se requiere " + rol})
			return
		}
		next(w, r)
	})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := ipCliente(r)
	if !s.limiter.permitido(ip) {
		auditar("login_bloqueado", r, "", "rate limit")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "demasiados intentos, espera 5 minutos"})
		return
	}

	var in struct {
		Usuario  string `json:"usuario"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalido"})
		return
	}
	in.Usuario = strings.TrimSpace(in.Usuario)
	if in.Usuario == "" || len(in.Usuario) > 64 || in.Password == "" || len(in.Password) > 72 {
		s.limiter.registrar(ip)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "credenciales invalidas"})
		return
	}

	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	var u user
	err := s.db.Collection("users").FindOne(ctx, bson.D{{Key: "usuario", Value: in.Usuario}}).Decode(&u)
	if err != nil {
		// Comparacion contra un hash ficticio: iguala el tiempo de respuesta y evita
		// enumerar usuarios por timing (A07).
		_ = bcrypt.CompareHashAndPassword(hashFicticio, []byte(in.Password))
		s.limiter.registrar(ip)
		auditar("login_fallido", r, in.Usuario, "usuario inexistente")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "credenciales invalidas"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(in.Password)) != nil {
		s.limiter.registrar(ip)
		auditar("login_fallido", r, in.Usuario, "password incorrecta")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "credenciales invalidas"})
		return
	}
	s.limiter.limpiar(ip)

	if !u.MfaActivo {
		secreto := u.Pendiente
		if secreto == "" {
			secreto = generarSecreto()
			_, err = s.db.Collection("users").UpdateOne(ctx,
				bson.D{{Key: "usuario", Value: u.Usuario}},
				bson.D{{Key: "$set", Value: bson.D{{Key: "pendiente", Value: secreto}}}})
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo preparar MFA"})
				return
			}
		}
		ticket, err := s.emitirToken(u.Usuario, u.Rol, "enroll", ticketTTL)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo emitir el ticket de enrolamiento"})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"estado":  "enroll",
			"usuario": u.Usuario,
			"secreto": secreto,
			"url":     otpauthURL(u.Usuario, secreto),
			"ticket":  ticket,
		})
		return
	}

	ticket, err := s.emitirToken(u.Usuario, u.Rol, "mfa", ticketTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo emitir el ticket MFA"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"estado": "mfa", "ticket": ticket})
}

func (s *server) handleVerify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Ticket string `json:"ticket"`
		Codigo string `json:"codigo"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalido"})
		return
	}
	in.Codigo = strings.TrimSpace(in.Codigo)
	if in.Codigo == "" || in.Ticket == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ticket y codigo son obligatorios"})
		return
	}

	c, err := s.validarToken(in.Ticket)
	if err != nil || (c.Tipo != "mfa" && c.Tipo != "enroll") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "ticket invalido o vencido"})
		return
	}
	// Limite por usuario: sin el, un ticket de 2 min permite fuerza bruta del TOTP (A07).
	if !s.mfaLimiter.permitido(c.Subject) {
		auditar("mfa_bloqueado", r, c.Subject, "rate limit")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "demasiados intentos, espera 5 minutos"})
		return
	}

	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	var u user
	if err := s.db.Collection("users").FindOne(ctx, bson.D{{Key: "usuario", Value: c.Subject}}).Decode(&u); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "ticket invalido o vencido"})
		return
	}
	// Un ticket "mfa" no sirve si el usuario aun no enrolo, ni uno "enroll" si ya lo hizo.
	if (c.Tipo == "mfa") != u.MfaActivo {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "ticket invalido o vencido"})
		return
	}

	secreto := u.TOTPSecret
	if c.Tipo == "enroll" {
		secreto = u.Pendiente
	}
	paso, ok := pasoTOTP(in.Codigo, secreto, u.UltimoPaso)
	if !ok {
		s.mfaLimiter.registrar(c.Subject)
		auditar("mfa_fallido", r, c.Subject, "codigo invalido o reutilizado")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "codigo TOTP invalido"})
		return
	}

	// Anti-replay: el paso TOTP se consume de forma atomica; un mismo codigo
	// no puede usarse dos veces aunque siga dentro de su ventana de 30 s.
	set := bson.D{{Key: "ultimoPasoTotp", Value: paso}}
	if c.Tipo == "enroll" {
		set = append(set,
			bson.E{Key: "totpSecret", Value: u.Pendiente},
			bson.E{Key: "pendiente", Value: ""},
			bson.E{Key: "mfaActivo", Value: true})
	}
	res, err := s.db.Collection("users").UpdateOne(ctx,
		bson.D{
			{Key: "usuario", Value: u.Usuario},
			{Key: "$or", Value: bson.A{
				bson.D{{Key: "ultimoPasoTotp", Value: bson.D{{Key: "$exists", Value: false}}}},
				bson.D{{Key: "ultimoPasoTotp", Value: bson.D{{Key: "$lt", Value: paso}}}},
			}},
		},
		bson.D{{Key: "$set", Value: set}})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo completar la verificacion"})
		return
	}
	if res.MatchedCount == 0 {
		s.mfaLimiter.registrar(c.Subject)
		auditar("mfa_fallido", r, c.Subject, "codigo reutilizado")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "codigo TOTP invalido"})
		return
	}
	s.mfaLimiter.limpiar(c.Subject)
	auditar("login_exitoso", r, u.Usuario, c.Tipo)

	token, err := s.emitirToken(u.Usuario, u.Rol, "acceso", accessTokenTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo emitir el token de acceso"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"usuario":  u.Usuario,
		"rol":      u.Rol,
		"expiraEn": time.Now().UTC().Add(accessTokenTTL).Format(time.RFC3339),
	})
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	usuario, rol := credenciales(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{"usuario": usuario, "rol": rol})
}

func seedUsers(ctx context.Context, db *mongo.Database) error {
	coll := db.Collection("users")
	count, err := coll.CountDocuments(ctx, bson.D{})
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	seed := func(usuario, password, rol string) error {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = coll.InsertOne(ctx, user{
			Usuario:  usuario,
			Hash:     string(hash),
			Rol:      rol,
			CreadoEn: time.Now().UTC().Format(time.RFC3339),
		})
		return err
	}
	// Las contraseñas iniciales vienen del entorno (secrets.env), nunca del codigo (A07).
	lectorPass := os.Getenv("SEED_LECTOR_PASSWORD")
	cambiadorPass := os.Getenv("SEED_CAMBIADOR_PASSWORD")
	if len(lectorPass) < 12 || len(cambiadorPass) < 12 {
		log.Printf("aviso: SEED_LECTOR_PASSWORD/SEED_CAMBIADOR_PASSWORD ausentes o < 12 caracteres; no se crean usuarios iniciales")
		return nil
	}
	if err := seed("lector1", lectorPass, rolLector); err != nil {
		return err
	}
	return seed("cambiador1", cambiadorPass, rolCambiador)
}

// hashFicticio se usa para igualar el tiempo de respuesta cuando el usuario no existe.
var hashFicticio, _ = bcrypt.GenerateFromPassword([]byte("usuario-inexistente-timing"), bcrypt.DefaultCost)

// pasoTOTP valida el codigo con tolerancia de +-1 periodo y devuelve el paso
// (contador de 30 s) que coincidio. Rechaza pasos ya consumidos (anti-replay).
func pasoTOTP(codigo, secreto string, ultimoPaso int64) (int64, bool) {
	if secreto == "" || len(codigo) != 6 {
		return 0, false
	}
	ahora := time.Now().UTC()
	for _, delta := range []int{-1, 0, 1} {
		t := ahora.Add(time.Duration(delta*totpPeriodo) * time.Second)
		paso := t.Unix() / totpPeriodo
		if paso <= ultimoPaso {
			continue
		}
		ok, err := totp.ValidateCustom(codigo, secreto, t, totp.ValidateOpts{
			Period:    totpPeriodo,
			Skew:      0,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err == nil && ok {
			return paso, true
		}
	}
	return 0, false
}
