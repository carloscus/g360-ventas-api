package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	ErrBadCredentials = errors.New("credenciales invalidas")
	ErrBadToken       = errors.New("token invalido")
	ErrExpiredToken   = errors.New("token expirado")
)

type Manager struct {
	secret []byte
	ttl    time.Duration
	user   string
	pass   string
}

func New(secret, user, pass string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl, user: user, pass: pass}
}

func (m *Manager) Login(user, pass string) (string, time.Time, error) {
	if m.user == "" || m.pass == "" {
		return "", time.Time{}, ErrBadCredentials
	}
	u := sha256.Sum256([]byte(user))
	p := sha256.Sum256([]byte(pass))
	du := sha256.Sum256([]byte(m.user))
	dp := sha256.Sum256([]byte(m.pass))
	if subtle.ConstantTimeCompare(u[:], du[:]) != 1 ||
		subtle.ConstantTimeCompare(p[:], dp[:]) != 1 {
		return "", time.Time{}, ErrBadCredentials
	}
	exp := time.Now().Add(m.ttl)
	return m.sign(strconv.FormatInt(exp.Unix(), 10)), exp, nil
}

func (m *Manager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (m *Manager) Verify(token string) error {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return ErrBadToken
	}
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	want, err := hex.DecodeString(sig)
	if err != nil || subtle.ConstantTimeCompare(mac.Sum(nil), want) != 1 {
		return ErrBadToken
	}
	exp, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return ErrBadToken
	}
	if time.Now().Unix() > exp {
		return ErrExpiredToken
	}
	return nil
}

func (m *Manager) extract(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if k := r.Header.Get("X-API-Key"); k != "" {
		return k
	}
	return r.URL.Query().Get("token")
}

func (m *Manager) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := m.extract(r)
		if tok == "" {
			writeAuthError(w, "token requerido")
			return
		}
		if err := m.Verify(tok); err != nil {
			writeAuthError(w, err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
