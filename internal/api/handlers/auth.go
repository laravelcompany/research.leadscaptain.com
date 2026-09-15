package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"research-leads/internal/api/middleware"
)

// AuthConfig carries the UI login configuration onto the Server.
type AuthConfig struct {
	User   string
	Pass   string
	Secret []byte
}

// Enabled reports whether the login screen is active.
func (a *AuthConfig) Enabled() bool { return a != nil && a.User != "" && a.Pass != "" }

// Login authenticates against the env-configured credentials and sets the
// session cookie. POST /api/v1/auth/login {"username":"...","password":"..."}
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":{"code":"METHOD_NOT_ALLOWED","message":"POST only"}}`, http.StatusMethodNotAllowed)
		return
	}
	if !s.Auth.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "auth_required": false})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "BAD_REQUEST", "message": "invalid JSON body"}})
		return
	}
	userOK := subtle.ConstantTimeCompare([]byte(body.Username), []byte(s.Auth.User)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(body.Password), []byte(s.Auth.Pass)) == 1
	if !userOK || !passOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "INVALID_CREDENTIALS", "message": "wrong username or password"}})
		return
	}
	token := middleware.SignSessionToken(s.Auth.User, s.Auth.Secret, time.Now())
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(middleware.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "username": s.Auth.User})
}

// Me reports whether auth is required and whether the caller is logged in.
// GET /api/v1/auth/me
func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.Auth.Enabled() {
		json.NewEncoder(w).Encode(map[string]any{"auth_required": false, "authenticated": true})
		return
	}
	if user, ok := middleware.SessionUser(r, s.Auth.Secret); ok {
		json.NewEncoder(w).Encode(map[string]any{"auth_required": true, "authenticated": true, "username": user})
		return
	}
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{"auth_required": true, "authenticated": false})
}

// Logout clears the session cookie. POST /api/v1/auth/logout
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
