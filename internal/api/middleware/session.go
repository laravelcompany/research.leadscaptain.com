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

const SessionCookie = "rl_session"
const SessionTTL = 7 * 24 * time.Hour

func SessionSecret(explicit string) []byte {
	if explicit == "" {
		return nil
	}
	sum := sha256.Sum256([]byte("linkedin-session:" + explicit))
	return sum[:]
}
func SignSessionToken(userID string, secret []byte, now time.Time) string {
	exp := now.Add(SessionTTL).Unix()
	payload := userID + "." + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func VerifySessionToken(token string, secret []byte, now time.Time) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(secret) == 0 {
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
func SessionUser(r *http.Request, secret []byte) (string, bool) {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		return "", false
	}
	return VerifySessionToken(c.Value, secret, time.Now())
}
func authExcluded(path string) bool {
	return path == "/health" || path == "/health/live" || path == "/health/ready" || path == "/metrics" || path == "/api/v1/auth/login" || path == "/api/v1/auth/callback" || path == "/api/v1/auth/me" || path == "/api/v1/auth/logout"
}
func Session(secret []byte, apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authExcluded(r.URL.Path) || !strings.HasPrefix(r.URL.Path, "/api/") {
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
			w.Write([]byte(`{"error":{"code":"AUTH_REQUIRED","message":"LinkedIn sign-in required"}}`))
		})
	}
}
