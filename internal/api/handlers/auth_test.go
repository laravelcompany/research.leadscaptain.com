package handlers

import (
	"database/sql"
	"golang.org/x/oauth2"
	_ "modernc.org/sqlite"
	"net/http"
	"net/http/httptest"
	"research-leads/internal/api/middleware"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testAuth() *AuthConfig {
	return &AuthConfig{ClientID: "client", ClientSecret: "secret", RedirectURL: "https://research.leadscaptain.com/api/v1/auth/callback", IssuerURL: "https://www.linkedin.com/oauth", Scopes: []string{"openid", "profile", "email"}, Secret: middleware.SessionSecret("session-secret")}
}
func testDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY AUTOINCREMENT,linkedin_sub TEXT NOT NULL UNIQUE,email TEXT,name TEXT NOT NULL DEFAULT '',picture_url TEXT,access_token TEXT,refresh_token TEXT,token_expires_at DATETIME,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,last_login_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
func TestAuthConfigValidation(t *testing.T) {
	if err := (&AuthConfig{}).ValidationError(); err == nil || !strings.Contains(err.Error(), "LINKEDIN_CLIENT_ID") {
		t.Fatalf("unexpected %v", err)
	}
	if err := testAuth().ValidationError(); err != nil {
		t.Fatal(err)
	}
}
func TestUpsertLinkedInUserCreatesAndUpdates(t *testing.T) {
	db := testDB(t)
	claims := struct{ Sub, Name, Email, Picture, Nonce string }{Sub: "abc", Name: "A", Email: "a@example.com"}
	id, err := upsertLinkedInUser(t.Context(), db, claims, &oauth2.Token{AccessToken: "one", Expiry: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	claims.Name = "Alice"
	id2, err := upsertLinkedInUser(t.Context(), db, claims, &oauth2.Token{AccessToken: "two"})
	if err != nil || id != id2 {
		t.Fatalf("id changed: %v %v", id2, err)
	}
	var name, token string
	if err = db.QueryRow("SELECT name,access_token FROM users WHERE id=?", id).Scan(&name, &token); err != nil || name != "Alice" || token != "two" {
		t.Fatalf("%q %q %v", name, token, err)
	}
}
func TestMeAndLogout(t *testing.T) {
	db := testDB(t)
	res, _ := db.Exec("INSERT INTO users(linkedin_sub,name,email) VALUES('sub','Alice','a@example.com')")
	id, _ := res.LastInsertId()
	s := &Server{DB: db, Auth: testAuth()}
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookie, Value: middleware.SignSessionToken(strconv.FormatInt(id, 10), s.Auth.Secret, time.Now())})
	w := httptest.NewRecorder()
	s.Me(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Alice") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.Logout(w, httptest.NewRequest("POST", "/api/v1/auth/logout", nil))
	if len(w.Result().Cookies()) == 0 || w.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatal("logout did not clear session")
	}
}
func TestMeRejectsMissingSession(t *testing.T) {
	s := &Server{DB: testDB(t), Auth: testAuth()}
	w := httptest.NewRecorder()
	s.Me(w, httptest.NewRequest("GET", "/api/v1/auth/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", w.Code)
	}
}
