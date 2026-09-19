package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"research-leads/internal/api/middleware"
)

const oauthFlowCookie = "rl_oauth_flow"

type AuthConfig struct {
	ClientID, ClientSecret, RedirectURL, IssuerURL string
	Scopes                                         []string
	Secret                                         []byte
}

func (a *AuthConfig) ValidationError() error {
	var missing []string
	if a == nil {
		return errors.New("LinkedIn authentication is not configured")
	}
	if a.ClientID == "" {
		missing = append(missing, "LINKEDIN_CLIENT_ID")
	}
	if a.ClientSecret == "" {
		missing = append(missing, "LINKEDIN_CLIENT_SECRET")
	}
	if a.RedirectURL == "" {
		missing = append(missing, "LINKEDIN_REDIRECT_URL")
	}
	if len(a.Secret) == 0 {
		missing = append(missing, "AUTH_SESSION_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	u, err := url.Parse(a.RedirectURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("LINKEDIN_REDIRECT_URL must be an absolute URL")
	}
	return nil
}
func (a *AuthConfig) oauthConfig(endpoint oauth2.Endpoint) *oauth2.Config {
	return &oauth2.Config{ClientID: a.ClientID, ClientSecret: a.ClientSecret, RedirectURL: a.RedirectURL, Scopes: a.Scopes, Endpoint: endpoint}
}
func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func signFlow(payload string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
func verifyFlow(v string, secret []byte) ([]string, bool) {
	p := strings.Split(v, ".")
	if len(p) != 4 {
		return nil, false
	}
	raw := strings.Join(p[:3], ".")
	expected := signFlow(raw, secret)
	if !hmac.Equal([]byte(expected), []byte(v)) {
		return nil, false
	}
	ts, e := strconv.ParseInt(p[2], 10, 64)
	return p[:3], e == nil && time.Since(time.Unix(ts, 0)) < 10*time.Minute
}
func secureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
func clearFlow(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: oauthFlowCookie, Value: "", Path: "/api/v1/auth", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.ValidationError(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	provider, err := oidc.NewProvider(r.Context(), s.Auth.IssuerURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "LinkedIn authentication is temporarily unavailable")
		return
	}
	state, _ := randomURLSafe(24)
	nonce, _ := randomURLSafe(24)
	verifier := oauth2.GenerateVerifier()
	flow := signFlow(state+"."+nonce+"."+strconv.FormatInt(time.Now().Unix(), 10), s.Auth.Secret)
	http.SetCookie(w, &http.Cookie{Name: oauthFlowCookie, Value: flow + "." + verifier, Path: "/api/v1/auth", MaxAge: 600, HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteLaxMode})
	cfg := s.Auth.oauthConfig(provider.Endpoint())
	http.Redirect(w, r, cfg.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}
func (s *Server) Callback(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.ValidationError(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	c, err := r.Cookie(oauthFlowCookie)
	if err != nil {
		writeError(w, http.StatusBadRequest, "OAuth login state is missing or expired")
		return
	}
	defer clearFlow(w)
	pieces := strings.Split(c.Value, ".")
	if len(pieces) != 5 {
		writeError(w, http.StatusBadRequest, "invalid OAuth login state")
		return
	}
	flow, ok := verifyFlow(strings.Join(pieces[:4], "."), s.Auth.Secret)
	if !ok || r.URL.Query().Get("state") != flow[0] {
		writeError(w, http.StatusBadRequest, "invalid OAuth state")
		return
	}
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		writeError(w, http.StatusBadRequest, "LinkedIn sign-in was cancelled or denied")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "LinkedIn did not return an authorization code")
		return
	}
	provider, err := oidc.NewProvider(r.Context(), s.Auth.IssuerURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "LinkedIn authentication is temporarily unavailable")
		return
	}
	cfg := s.Auth.oauthConfig(provider.Endpoint())
	tok, err := cfg.Exchange(r.Context(), code, oauth2.VerifierOption(pieces[4]))
	if err != nil {
		writeError(w, http.StatusBadRequest, "LinkedIn authorization code could not be validated")
		return
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok {
		writeError(w, http.StatusBadRequest, "LinkedIn response did not include an ID token")
		return
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: s.Auth.ClientID}).Verify(r.Context(), rawID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "LinkedIn ID token could not be validated")
		return
	}
	var claims struct{ Sub, Name, Email, Picture, Nonce string }
	if err = idToken.Claims(&claims); err != nil || claims.Sub == "" || claims.Nonce != flow[1] {
		writeError(w, http.StatusBadRequest, "LinkedIn identity response is invalid")
		return
	}
	id, err := upsertLinkedInUser(r.Context(), s.DB, claims, tok)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save LinkedIn user")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: middleware.SessionCookie, Value: middleware.SignSessionToken(strconv.FormatInt(id, 10), s.Auth.Secret, time.Now()), Path: "/", MaxAge: int(middleware.SessionTTL.Seconds()), HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusFound)
}
func upsertLinkedInUser(ctx context.Context, db *sql.DB, c struct{ Sub, Name, Email, Picture, Nonce string }, tok *oauth2.Token) (int64, error) {
	_, err := db.ExecContext(ctx, `INSERT INTO users(linkedin_sub,email,name,picture_url,access_token,refresh_token,token_expires_at,last_login_at) VALUES(?,?,?,?,?,?,?,CURRENT_TIMESTAMP) ON CONFLICT(linkedin_sub) DO UPDATE SET email=excluded.email,name=excluded.name,picture_url=excluded.picture_url,access_token=excluded.access_token,refresh_token=excluded.refresh_token,token_expires_at=excluded.token_expires_at,updated_at=CURRENT_TIMESTAMP,last_login_at=CURRENT_TIMESTAMP`, c.Sub, c.Email, c.Name, c.Picture, tok.AccessToken, tok.RefreshToken, tok.Expiry)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.QueryRowContext(ctx, "SELECT id FROM users WHERE linkedin_sub=?", c.Sub).Scan(&id)
	return id, err
}
func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := s.Auth.ValidationError(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{"auth_required": true, "authenticated": false, "configuration_error": err.Error()})
		return
	}
	uid, ok := middleware.SessionUser(r, s.Auth.Secret)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"auth_required": true, "authenticated": false})
		return
	}
	var name, email, picture string
	err := s.DB.QueryRow("SELECT name,COALESCE(email,''),COALESCE(picture_url,'') FROM users WHERE id=?", uid).Scan(&name, &email, &picture)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"auth_required": true, "authenticated": false})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"auth_required": true, "authenticated": true, "user": map[string]string{"name": name, "email": email, "picture": picture}})
}
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: middleware.SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteLaxMode})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
