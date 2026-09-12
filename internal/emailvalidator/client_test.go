package emailvalidator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestVerifyParsesStatusAndEscapesEmail(t *testing.T) {
	var gotEmail string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEmail = r.URL.Query().Get("email")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"risky"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", 5)
	res, err := c.Verify(context.Background(), "we ird+addr@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "risky" {
		t.Fatalf("expected status from response body, got %q", res.Status)
	}
	want, _ := url.QueryUnescape(url.QueryEscape("we ird+addr@example.com"))
	if gotEmail != want {
		t.Fatalf("email not transmitted correctly: %q", gotEmail)
	}
}

func TestVerifyUnknownOnUnexpectedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "", 5)
	res, err := c.Verify(context.Background(), "a@b.com")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "unknown" {
		t.Fatalf("expected unknown, got %q", res.Status)
	}
}

func TestVerifyHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c := New(srv.URL, "", 5)
	if _, err := c.Verify(context.Background(), "a@b.com"); err == nil {
		t.Fatal("expected error on HTTP 500")
	}
}
