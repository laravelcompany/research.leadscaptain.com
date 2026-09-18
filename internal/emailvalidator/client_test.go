package emailvalidator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyUsesLaravelMailContract(t *testing.T) {
	var gotMethod, gotPath, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"email":"person@example.com","verdict":{"status":"safe","score":0.946}}`))
	}))
	defer srv.Close()

	res, err := New(srv.URL, "", 5).Verify(context.Background(), "person@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/verify-email" || gotType != "application/json" {
		t.Fatalf("wrong API request: %s %s %s", gotMethod, gotPath, gotType)
	}
	if res.Status != "valid" || res.Score != 95 || res.Email != "person@example.com" {
		t.Fatalf("wrong mapped result: %+v", res)
	}
}

func TestVerifyMapsLaravelMailVerdicts(t *testing.T) {
	for remote, want := range map[string]string{"safe": "valid", "risky": "risky", "unknown": "unknown", "invalid": "invalid"} {
		t.Run(remote, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"verdict":{"status":"` + remote + `","score":0.5}}`))
			}))
			defer srv.Close()
			got, err := New(srv.URL, "", 5).Verify(context.Background(), "person@example.com")
			if err != nil || got.Status != want {
				t.Fatalf("got %+v, %v; want %s", got, err, want)
			}
		})
	}
}

func TestVerifyFallsBackLocally(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"rate limited": func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "60"); w.WriteHeader(429) },
		"server error": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) },
		"malformed":    func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{`)) },
		"unsupported verdict": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"verdict":{"status":"accepted","score":0.9}}`))
		},
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(handler)
			defer srv.Close()
			got, err := New(srv.URL, "", 5).Verify(context.Background(), "not-an-email")
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "invalid" || got.Score != 0 {
				t.Fatalf("local fallback not used: %+v", got)
			}
		})
	}
}

func TestVerifyFallsBackOnNetworkFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	got, err := New(url, "", 1).Verify(context.Background(), "not-an-email")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "invalid" {
		t.Fatalf("local fallback not used: %+v", got)
	}
}

func TestVerifyPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New("http://127.0.0.1:1", "", 1).Verify(ctx, "not-an-email")
	if err != context.Canceled {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestVerifyWithEmptyURLUsesLocalVerifier(t *testing.T) {
	got, err := New("", "", 1).Verify(context.Background(), "not-an-email")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "invalid" || got.Score != 0 {
		t.Fatalf("got %+v", got)
	}
}
