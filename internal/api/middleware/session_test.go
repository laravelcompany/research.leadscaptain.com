package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okHandler(hit *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { *hit = true })
}
func TestSessionTokenRoundTrip(t *testing.T) {
	secret := SessionSecret("secret")
	tok := SignSessionToken("42", secret, time.Now())
	u, ok := VerifySessionToken(tok, secret, time.Now())
	if !ok || u != "42" {
		t.Fatalf("round trip %q %v", u, ok)
	}
}
func TestSessionTokenTamperedAndExpired(t *testing.T) {
	secret := SessionSecret("secret")
	tok := SignSessionToken("42", secret, time.Now())
	if _, ok := VerifySessionToken(tok+"x", secret, time.Now()); ok {
		t.Fatal("tampered token accepted")
	}
	tok = SignSessionToken("42", secret, time.Now().Add(-2*SessionTTL))
	if _, ok := VerifySessionToken(tok, secret, time.Now()); ok {
		t.Fatal("expired token accepted")
	}
}
func TestSessionBlocksAndAllowsCookie(t *testing.T) {
	secret := SessionSecret("secret")
	hit := false
	h := Session(secret, "")(okHandler(&hit))
	r := httptest.NewRequest("GET", "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if hit || w.Code != 401 {
		t.Fatalf("wanted 401, hit=%v code=%d", hit, w.Code)
	}
	r = httptest.NewRequest("GET", "/api/v1/stats", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: SignSessionToken("42", secret, time.Now())})
	h.ServeHTTP(httptest.NewRecorder(), r)
	if !hit {
		t.Fatal("valid session rejected")
	}
}
func TestSessionAllowsAPIKeyAndPublicPaths(t *testing.T) {
	for _, tc := range []struct{ path, key string }{{"/", ""}, {"/api/v1/auth/login", ""}, {"/api/v1/auth/callback", ""}, {"/health", ""}, {"/api/v1/stats", "key"}} {
		hit := false
		h := Session(SessionSecret("secret"), "key")(okHandler(&hit))
		r := httptest.NewRequest("GET", tc.path, nil)
		if tc.key != "" {
			r.Header.Set("Authorization", "Bearer "+tc.key)
		}
		h.ServeHTTP(httptest.NewRecorder(), r)
		if !hit {
			t.Fatalf("%s rejected", tc.path)
		}
	}
}
