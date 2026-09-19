package handlers

import (
	"database/sql"
	"golang.org/x/oauth2"
	_ "modernc.org/sqlite"
	"net/http"
	"net/http/httptest"
	"net/url"
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
func TestMeReturnsSignedOutStateWithoutTurningJSONIntoAnError(t *testing.T) {
	s := &Server{DB: testDB(t), Auth: testAuth()}
	w := httptest.NewRecorder()
	s.Me(w, httptest.NewRequest("GET", "/api/v1/auth/me", nil))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/json" || !strings.Contains(w.Body.String(), `"authenticated":false`) {
		t.Fatalf("got %d %s %s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
}

func TestLinkedInConfidentialClientTokenExchangeUsesOneFormRequestAndConfiguredRedirect(t *testing.T) {
	requests := 0
	var gotRedirect, gotClientID, gotClientSecret, gotCode, gotVerifier string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotRedirect = r.Form.Get("redirect_uri")
		gotClientID = r.Form.Get("client_id")
		gotClientSecret = r.Form.Get("client_secret")
		gotCode = r.Form.Get("code")
		gotVerifier = r.Form.Get("code_verifier")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	auth := testAuth()
	cfg := auth.oauthConfig(oauth2.Endpoint{AuthURL: "https://www.linkedin.com/oauth/v2/authorization", TokenURL: tokenServer.URL})
	if !strings.Contains(cfg.AuthCodeURL("state"), "redirect_uri="+url.QueryEscape(auth.RedirectURL)) {
		t.Fatalf("authorization request does not use configured redirect URI")
	}
	if _, err := cfg.Exchange(t.Context(), "single-use-code"); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("token endpoint received %d requests; authorization codes are single-use", requests)
	}
	if gotRedirect != auth.RedirectURL || gotClientID != auth.ClientID || gotClientSecret != auth.ClientSecret || gotCode != "single-use-code" || gotVerifier != "" {
		t.Fatalf("unexpected confidential-client token form: redirect=%q client=%q secret=%q code=%q verifier=%q", gotRedirect, gotClientID, gotClientSecret, gotCode, gotVerifier)
	}
}
