package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPIKeyDisabled(t *testing.T) {
	hit := false
	h := APIKey("", nil)(okHandler(&hit))
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hit {
		t.Fatal("empty key should disable the gate")
	}
}

func TestAPIKeyStaticPathsOpen(t *testing.T) {
	for _, path := range []string{"/", "/index.html", "/assets/index-abc123.js", "/favicon.svg", "/some/spa/route"} {
		hit := false
		h := APIKey("api-key-123", nil)(okHandler(&hit))
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if !hit {
			t.Fatalf("static path %s must stay public so the login screen loads", path)
		}
		if rec.Code == http.StatusUnauthorized {
			t.Fatalf("static path %s returned 401", path)
		}
	}
}

func TestAPIKeyGatesAPIRoutes(t *testing.T) {
	h := APIKey("api-key-123", nil)(okHandler(new(bool)))
	for _, path := range []string{"/api/v1/stats", "/api/v1/leads", "/api/v1/events"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s without key: got %d, want 401", path, rec.Code)
		}
	}
}

func TestAPIKeyAcceptsBearer(t *testing.T) {
	hit := false
	h := APIKey("api-key-123", nil)(okHandler(&hit))
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer api-key-123")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hit {
		t.Fatal("valid bearer token should pass")
	}
}

func TestAPIKeySessionCookieOnlyForAPI(t *testing.T) {
	hit := false
	secret := SessionSecret("secret")
	h := APIKey("api-key-123", secret)(okHandler(&hit))
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: SignSessionToken("42", secret, time.Now())})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hit {
		t.Fatal("valid session cookie should satisfy the API gate")
	}
}
