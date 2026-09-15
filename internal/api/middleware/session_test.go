package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionTokenRoundTrip(t *testing.T) {
	secret := SessionSecret("admin", "s3cret", "")
	tok := SignSessionToken("admin", secret, time.Now())
	user, ok := VerifySessionToken(tok, secret, time.Now())
	if !ok || user != "admin" {
		t.Fatalf("round trip failed: user=%q ok=%v", user, ok)
	}
}

func TestSessionTokenTampered(t *testing.T) {
	secret := SessionSecret("admin", "s3cret", "")
	tok := SignSessionToken("admin", secret, time.Now())
	if _, ok := VerifySessionToken(tok[:len(tok)-2]+"xx", secret, time.Now()); ok {
		t.Fatal("tampered signature accepted")
	}
	other := SessionSecret("admin", "different", "")
	if _, ok := VerifySessionToken(tok, other, time.Now()); ok {
		t.Fatal("token accepted under a different secret")
	}
}

func TestSessionTokenExpired(t *testing.T) {
	secret := SessionSecret("admin", "s3cret", "")
	past := time.Now().Add(-2 * SessionTTL)
	tok := SignSessionToken("admin", secret, past)
	if _, ok := VerifySessionToken(tok, secret, time.Now()); ok {
		t.Fatal("expired token accepted")
	}
}

func TestExplicitSecretOverridesDerived(t *testing.T) {
	a := SessionSecret("admin", "pass", "my-secret")
	b := SessionSecret("admin", "pass", "")
	if string(a) == string(b) {
		t.Fatal("explicit secret should change the key")
	}
}

func okHandler(hit *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { *hit = true })
}

func TestSessionMiddlewareDisabled(t *testing.T) {
	hit := false
	h := Session("", "", nil, "")(okHandler(&hit))
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hit {
		t.Fatal("disabled auth should pass requests through")
	}
}

func TestSessionMiddlewareBlocksWithoutCookie(t *testing.T) {
	hit := false
	secret := SessionSecret("admin", "pass", "")
	h := Session("admin", "pass", secret, "")(okHandler(&hit))
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if hit || rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got hit=%v code=%d", hit, rec.Code)
	}
}

func TestSessionMiddlewareAllowsCookie(t *testing.T) {
	hit := false
	secret := SessionSecret("admin", "pass", "")
	h := Session("admin", "pass", secret, "")(okHandler(&hit))
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: SignSessionToken("admin", secret, time.Now())})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hit {
		t.Fatal("valid session cookie rejected")
	}
}

func TestSessionMiddlewareAllowsBearerKey(t *testing.T) {
	hit := false
	secret := SessionSecret("admin", "pass", "")
	h := Session("admin", "pass", secret, "api-key-123")(okHandler(&hit))
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer api-key-123")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hit {
		t.Fatal("valid bearer key rejected by session middleware")
	}
}

func TestSessionMiddlewareLeavesStaticPublic(t *testing.T) {
	hit := false
	secret := SessionSecret("admin", "pass", "")
	h := Session("admin", "pass", secret, "")(okHandler(&hit))
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hit {
		t.Fatal("static frontend should stay public so the login screen loads")
	}
}

func TestAPIKeyAcceptsSessionCookie(t *testing.T) {
	hit := false
	secret := SessionSecret("admin", "pass", "")
	h := APIKey("api-key-123", secret)(okHandler(&hit))
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: SignSessionToken("admin", secret, time.Now())})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hit {
		t.Fatal("APIKey should accept a valid UI session cookie")
	}
}

func TestAPIKeyLoginPathOpen(t *testing.T) {
	hit := false
	h := APIKey("api-key-123", nil)(okHandler(&hit))
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !hit {
		t.Fatal("login endpoint must stay reachable when APP_API_KEY is set")
	}
}
