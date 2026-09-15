package middleware

import "net/http"

// APIKey gates requests behind the APP_API_KEY Bearer token. A valid UI
// session cookie (see Session) also satisfies the check so the React app
// keeps working when both are configured.
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
			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing api key"}}`, 401)
		})
	}
}
