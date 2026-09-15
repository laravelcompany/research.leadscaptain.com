package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"research-leads/internal/api/middleware"
)

func testAuth() *AuthConfig {
	return &AuthConfig{User: "admin", Pass: "s3cret", Secret: middleware.SessionSecret("admin", "s3cret", "")}
}

func TestLoginSuccessSetsCookie(t *testing.T) {
	s := &Server{Auth: testAuth()}
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"s3cret"}`))
	rec := httptest.NewRecorder()
	s.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.SessionCookie {
			session = c
		}
	}
	if session == nil || session.Value == "" || !session.HttpOnly {
		t.Fatal("expected an HttpOnly session cookie")
	}
	if _, ok := middleware.VerifySessionToken(session.Value, s.Auth.Secret, time.Now()); !ok {
		t.Fatal("cookie token does not verify")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	s := &Server{Auth: testAuth()}
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	rec := httptest.NewRecorder()
	s.Login(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLoginRejectsWrongUsername(t *testing.T) {
	s := &Server{Auth: testAuth()}
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"username":"root","password":"s3cret"}`))
	rec := httptest.NewRecorder()
	s.Login(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMeDisabledAuth(t *testing.T) {
	s := &Server{Auth: &AuthConfig{}}
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	s.Me(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["auth_required"] != false {
		t.Fatalf("expected auth_required=false, got %v", body)
	}
}

func TestMeUnauthenticated(t *testing.T) {
	s := &Server{Auth: testAuth()}
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	s.Me(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	s := &Server{Auth: testAuth()}
	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	s.Logout(rec, req)
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.SessionCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("logout should clear the session cookie")
	}
}
