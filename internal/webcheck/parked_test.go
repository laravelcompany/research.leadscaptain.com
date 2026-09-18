package webcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckFlagsParkedDomain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>acme-test-example.com</title></head><body>This domain is for sale. Inquire about this domain today.</body></html>`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	res := Check(context.Background(), host)
	if !res.Reachable {
		t.Fatalf("test server must be reachable: %s", res.Error)
	}
	if !res.Parked {
		t.Fatal("for-sale placeholder page must be flagged as parked")
	}
}

func TestCheckRealSiteNotParked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Acme - Industrial widgets</title></head><body>We manufacture widgets since 1982.</body></html>`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	res := Check(context.Background(), host)
	if res.Parked {
		t.Fatal("normal company page must not be flagged as parked")
	}
}
