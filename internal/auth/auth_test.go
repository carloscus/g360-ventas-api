package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
