package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SessionCookie is the HttpOnly cookie that carries the UI login session.
const SessionCookie = "rl_session"

// SessionTTL is how long a login session stays valid.
const SessionTTL = 7 * 24 * time.Hour

// SessionSecret derives the HMAC key for session tokens. When no explicit
// AUTH_SESSION_SECRET is configured it is derived from the credentials, so
// tokens never depend on a random process lifetime.
func SessionSecret(user, pass, explicit string) []byte {
	if explicit != "" {
		sum := sha256.Sum256([]byte("explicit:" + explicit))
		return sum[:]
	}
	sum := sha256.Sum256([]byte("creds:" + user + ":" + pass))
	return sum[:]
}

// SignSessionToken issues a token of the form user.expires.signature.
func SignSessionToken(user string, secret []byte, now time.Time) string {
	exp := now.Add(SessionTTL).Unix()
	payload := user + "." + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// VerifySessionToken validates a token and returns the username it carries.
func VerifySessionToken(token string, secret []byte, now time.Time) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	payload := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[2])) != 1 {
		return "", false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() > exp {
		return "", false
	}
	return parts[0], true
}

// SessionUser returns the authenticated username from the session cookie, if any.
func SessionUser(r *http.Request, secret []byte) (string, bool) {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		return "", false
	}
	return VerifySessionToken(c.Value, secret, time.Now())
}

// authExcluded paths stay reachable without a session (health probes and the
// login endpoints themselves).
func authExcluded(path string) bool {
	switch {
	case path == "/health" || path == "/health/live" || path == "/health/ready" || path == "/metrics":
		return true
	case path == "/api/v1/auth/login" || path == "/api/v1/auth/me":
		return true
	}
	return false
}

// Session gates every request behind a UI login when AUTH_USERNAME and
// AUTH_PASSWORD are configured. With either unset the middleware is a no-op
// (auth disabled). API clients using the APP_API_KEY Bearer token pass
// through untouched, and the static frontend files stay public so the login
// screen itself can load.
func Session(user, pass string, secret []byte, apiKey string) func(http.Handler) http.Handler {
	enabled := user != "" && pass != ""
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled || authExcluded(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			// Static assets (the SPA and its files) must load so the login
			// screen renders; the API below is what actually gates data.
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}
			if apiKey != "" && r.Header.Get("Authorization") == "Bearer "+apiKey {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := SessionUser(r, secret); ok {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"code":"AUTH_REQUIRED","message":"login required"}}`))
		})
	}
}
