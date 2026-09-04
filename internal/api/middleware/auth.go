package middleware

import "net/http"

func APIKey(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
			if key=="" { next.ServeHTTP(w,r); return }
			if r.URL.Path=="/health" || r.URL.Path=="/health/live" || r.URL.Path=="/health/ready" || r.URL.Path=="/metrics" { next.ServeHTTP(w,r); return }
			if r.Header.Get("Authorization")=="Bearer "+key { next.ServeHTTP(w,r); return }
			http.Error(w,`{"error":{"code":"UNAUTHORIZED","message":"missing api key"}}`,401)
		})
	}
}
