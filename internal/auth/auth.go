package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"g360-ventas-api/internal/config"
)

var (
	ErrBadCredentials = errors.New("credenciales invalidas")
	ErrBadToken       = errors.New("token invalido")
	ErrExpiredToken   = errors.New("token expirado")
)

type Manager struct {
	secret []byte
	ttl    time.Duration
	users  map[string]string
}

func New(secret, user, pass string, ttl time.Duration) *Manager {
	m := &Manager{secret: []byte(secret), ttl: ttl, users: make(map[string]string)}
	if user != "" {
		m.users[user] = pass
	}
	return m
}

func NewMulti(secret string, users []config.UserConfig, ttl time.Duration) *Manager {
	m := &Manager{secret: []byte(secret), ttl: ttl, users: make(map[string]string)}
	for _, u := range users {
		if u.User == "" {
			continue
		}
		m.users[u.User] = u.Pass
	}
	return m
}

func (m *Manager) UserCount() int {
	return len(m.users)
}

func (m *Manager) Login(user, pass string) (string, time.Time, error) {
	if len(m.users) == 0 {
		return "", time.Time{}, ErrBadCredentials
	}
	expected, ok := m.users[user]
	if !ok {
		// Comparacion dummy para no revelar que el usuario no existe por timing.
		dummy := sha256.Sum256([]byte("usuario-inexistente"))
		_ = subtle.ConstantTimeCompare(dummy[:], dummy[:])
		return "", time.Time{}, ErrBadCredentials
	}
	p := sha256.Sum256([]byte(pass))
	dp := sha256.Sum256([]byte(expected))
	if subtle.ConstantTimeCompare(p[:], dp[:]) != 1 {
		return "", time.Time{}, ErrBadCredentials
	}
	exp := time.Now().Add(m.ttl)
	payload := fmt.Sprintf("%s.%d", user, exp.Unix())
	return m.sign(payload), exp, nil
}

func (m *Manager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (m *Manager) Verify(token string) error {
	_, err := m.VerifyUser(token)
	return err
}

func (m *Manager) VerifyUser(token string) (string, error) {
	idx := strings.LastIndex(token, ".")
	if idx < 0 {
		return "", ErrBadToken
	}
	payload, sig := token[:idx], token[idx+1:]
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	want, err := hex.DecodeString(sig)
	if err != nil || subtle.ConstantTimeCompare(mac.Sum(nil), want) != 1 {
		return "", ErrBadToken
	}
	var expStr, user string
	if i := strings.LastIndex(payload, "."); i >= 0 {
		user, expStr = payload[:i], payload[i+1:]
	} else {
		expStr = payload
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", ErrBadToken
	}
	if time.Now().Unix() > exp {
		return "", ErrExpiredToken
	}
	return user, nil
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
