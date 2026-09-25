package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"net"
	"net/http"
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
)

const (
	maxLoginIntentos = 10
	ventanaLogin     = 5 * time.Minute
)

type user struct {
	Usuario    string `bson:"usuario" json:"usuario"`
	Hash       string `bson:"hash" json:"-"`
	Rol        string `bson:"rol" json:"rol"`
	TOTPSecret string `bson:"totpSecret,omitempty" json:"-"`
	Pendiente  string `bson:"pendiente,omitempty" json:"-"`
	MfaActivo  bool   `bson:"mfaActivo" json:"mfaActivo"`
	CreadoEn   string `bson:"creadoEn" json:"creadoEn"`
}

type ctxKey string

const (
	ctxUsuarioKey ctxKey = "usuario"
	ctxRolKey     ctxKey = "rol"
)

type authClaims struct {
	Rol  string `json:"rol,omitempty"`
	Tipo string `json:"tipo,omitempty"` // "acceso" | "mfa" | "enroll"
	jwt.RegisteredClaims
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: map[string][]time.Time{}}
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
	l.attempts[ip] = vivos
	return len(vivos) < maxLoginIntentos
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

func ipCliente(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		partes := strings.Split(xff, ",")
		if len(partes) > 0 {
			return strings.TrimSpace(partes[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
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
		tokenStr := bearerDe(r)
		if tokenStr == "" {
			tokenStr = r.URL.Query().Get("token")
		}
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

func (s *server) requireRole(rol string, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if _, rolActual := credenciales(r.Context()); rolActual != rol {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "rol insuficiente: se requiere " + rol})
			return
		}
		next(w, r)
	})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := ipCliente(r)
	if !s.limiter.permitido(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "demasiados intentos, espera 5 minutos"})
		return
	}

	var in struct {
		Usuario  string `json:"usuario"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalido"})
		return
	}
	in.Usuario = strings.TrimSpace(in.Usuario)

	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	var u user
	err := s.db.Collection("users").FindOne(ctx, bson.D{{Key: "usuario", Value: in.Usuario}}).Decode(&u)
	if err != nil {
		s.limiter.registrar(ip)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "credenciales invalidas"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(in.Password)) != nil {
		s.limiter.registrar(ip)
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
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
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

	ctx, cancel := s.ctx(r.Context())
	defer cancel()

	var u user
	if err := s.db.Collection("users").FindOne(ctx, bson.D{{Key: "usuario", Value: c.Subject}}).Decode(&u); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "usuario inexistente"})
		return
	}

	secreto := u.TOTPSecret
	if c.Tipo == "enroll" {
		secreto = u.Pendiente
	}
	if secreto == "" || !totp.Validate(in.Codigo, secreto) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "codigo TOTP invalido"})
		return
	}

	if c.Tipo == "enroll" {
		_, err = s.db.Collection("users").UpdateOne(ctx,
			bson.D{{Key: "usuario", Value: u.Usuario}},
			bson.D{{Key: "$set", Value: bson.D{
				{Key: "totpSecret", Value: u.Pendiente},
				{Key: "pendiente", Value: ""},
				{Key: "mfaActivo", Value: true},
			}}})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo activar MFA"})
			return
		}
		u.Rol = c.Rol
	}

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
	if err := seed("lector1", "LectorLab123!", rolLector); err != nil {
		return err
	}
	return seed("cambiador1", "CambiadorLab123!", rolCambiador)
}
