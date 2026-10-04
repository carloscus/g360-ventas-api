package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"g360-ventas-api/internal/config"
)

func TestLoginAndVerify(t *testing.T) {
	m := New("secret", "user1", "pass1", time.Hour)

	tok, exp, err := m.Login("user1", "pass1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if exp.Before(time.Now()) {
		t.Error("expiracion en el pasado")
	}
	if err := m.Verify(tok); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestLoginBadCredentials(t *testing.T) {
	m := New("secret", "user1", "pass1", time.Hour)
	if _, _, err := m.Login("user1", "mala"); err != ErrBadCredentials {
		t.Errorf("err = %v", err)
	}
	if _, _, err := m.Login("otro", "pass1"); err != ErrBadCredentials {
		t.Errorf("err = %v", err)
	}
}

func TestVerifyTampered(t *testing.T) {
	m := New("secret", "user1", "pass1", time.Hour)
	tok, _, err := m.Login("user1", "pass1")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Verify(tok + "x"); err != ErrBadToken {
		t.Errorf("tamper = %v", err)
	}
	if err := m.Verify("basura"); err != ErrBadToken {
		t.Errorf("basura = %v", err)
	}
	other := New("otro-secret", "user1", "pass1", time.Hour)
	if err := other.Verify(tok); err != ErrBadToken {
		t.Errorf("secret distinto = %v", err)
	}
}

func TestVerifyExpired(t *testing.T) {
	m := New("secret", "user1", "pass1", -time.Second)
	tok, _, err := m.Login("user1", "pass1")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Verify(tok); err != ErrExpiredToken {
		t.Errorf("expirado = %v", err)
	}
}

func TestRequireMiddleware(t *testing.T) {
	m := New("secret", "user1", "pass1", time.Hour)
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := m.Require(ok)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/x", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("sin token = %d", rec.Code)
	}

	tok, _, _ := m.Login("user1", "pass1")

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("bearer = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/x?token="+tok, nil))
	if rec.Code != http.StatusOK {
		t.Errorf("query token = %d", rec.Code)
	}
}

func TestMultiUserLogin(t *testing.T) {
	m := NewMulti("secret", []config.UserConfig{
		{User: "ccusi", Pass: "123456"},
		{User: "cliente1", Pass: "clave1"},
	}, time.Hour)
	if m.UserCount() != 2 {
		t.Fatalf("usuarios = %d, quiero 2", m.UserCount())
	}

	tok1, _, err := m.Login("ccusi", "123456")
	if err != nil {
		t.Fatalf("login ccusi: %v", err)
	}
	tok2, _, err := m.Login("cliente1", "clave1")
	if err != nil {
		t.Fatalf("login cliente1: %v", err)
	}
	if tok1 == tok2 {
		t.Error("tokens de usuarios distintos no deberian coincidir")
	}
	if err := m.Verify(tok1); err != nil {
		t.Errorf("verify tok1: %v", err)
	}
	if err := m.Verify(tok2); err != nil {
		t.Errorf("verify tok2: %v", err)
	}
	if u, err := m.VerifyUser(tok1); err != nil || u != "ccusi" {
		t.Errorf("VerifyUser tok1 = %q, %v", u, err)
	}
	if u, err := m.VerifyUser(tok2); err != nil || u != "cliente1" {
		t.Errorf("VerifyUser tok2 = %q, %v", u, err)
	}
	// Credenciales cruzadas deben fallar.
	if _, _, err := m.Login("ccusi", "clave1"); err != ErrBadCredentials {
		t.Errorf("cruce pass = %v", err)
	}
	if _, _, err := m.Login("nadie", "123456"); err != ErrBadCredentials {
		t.Errorf("usuario inexistente = %v", err)
	}
}

func TestVerifyLegacyToken(t *testing.T) {
	m := New("secret", "user1", "pass1", time.Hour)
	tok, _, err := m.Login("user1", "pass1")
	if err != nil {
		t.Fatal(err)
	}
	// El token nuevo lleva username; Verify clasico debe seguir aceptandolo.
	if err := m.Verify(tok); err != nil {
		t.Fatalf("verify token con usuario: %v", err)
	}
}
