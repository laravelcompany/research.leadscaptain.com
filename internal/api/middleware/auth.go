package middleware

import (
	"net/http"
	"strings"
)

// APIKey gates API requests behind the APP_API_KEY Bearer token. A valid LinkedIn browser
// session cookie (see Session) also satisfies the check so the React app
// keeps working when both are configured. Static frontend files and the SPA
// fallback stay public: the key gates API data, not the LinkedIn connect screen, which
// must load before any session or key exists.
func APIKey(key string, sessionSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if authExcluded(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			// Only API routes are gated. Everything else is the SPA, its
			// assets, or the index.html fallback - all safe to serve because
			// they carry no lead data.
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("Authorization") == "Bearer "+key {
				next.ServeHTTP(w, r)
				return
			}
			if len(sessionSecret) > 0 {
				if _, ok := SessionUser(r, sessionSecret); ok {
					next.ServeHTTP(w, r)
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing api key"}}`, http.StatusUnauthorized)
		})
	}
}
