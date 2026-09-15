package companyreg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSplitVAT(t *testing.T) {
	cc, num := splitVAT("DE123456789")
	if cc != "DE" || num != "123456789" {
		t.Fatalf("got %q %q", cc, num)
	}
	cc, num = splitVAT("123456789")
	if cc != "" || num != "123456789" {
		t.Fatalf("bare number must have empty country, got %q %q", cc, num)
	}
	cc, num = splitVAT("RO  999 ")
	if cc != "RO" || num != "999" {
		t.Fatalf("spaces must be tolerated, got %q %q", cc, num)
	}
}

func TestRoutingFailsSafe(t *testing.T) {
	c := New("", 5)
	cases := []struct {
		q       Query
		wantErr string
	}{
		{Query{Name: "Acme"}, "needs country"},
		{Query{Name: "Acme", Country: "DE"}, "vat_number"},
		{Query{Name: "Acme", Country: "XX"}, "unsupported country"},
		{Query{Country: "RO", Name: "Acme"}, "fiscal_code"},
		{Query{VATNumber: "GB123456789"}, "does not cover GB"},
		{Query{VATNumber: "123456789"}, "no country prefix"},
		{Query{Name: "Acme", Country: "GB"}, "COMPANIES_HOUSE_API_KEY"},
	}
	for _, tc := range cases {
		_, err := c.Lookup(context.Background(), tc.q)
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%+v: expected error containing %q, got %v", tc.q, tc.wantErr, err)
		}
	}
	// UK normalises to GB.
	_, err := c.Lookup(context.Background(), Query{Name: "Acme", Country: "UK"})
	if err == nil || !strings.Contains(err.Error(), "COMPANIES_HOUSE_API_KEY") {
		t.Fatalf("UK must route to Companies House, got %v", err)
	}
}

func TestVIESLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/ms/DE/vat/123456789") {
			t.Errorf("unexpected VIES path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"isValid":true,"name":"ACME GMBH","address":"HAUPTSTRASSE 1\n10115 BERLIN","userError":"VALID"}`)
	}))
	defer srv.Close()
	c := New("", 5)
	c.viesURL = srv.URL
	res, err := c.Lookup(context.Background(), Query{VATNumber: "DE123456789"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != "vies" || len(res.Companies) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	co := res.Companies[0]
	if *co.VATValid != true || co.Name != "ACME GMBH" {
		t.Fatalf("unexpected company: %+v", co)
	}
	if strings.Contains(co.Address, "\n") {
		t.Fatalf("address must be normalised, got %q", co.Address)
	}
}

func TestVIESInvalidNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"isValid":false,"name":"---","address":"---","userError":"VALID"}`)
	}))
	defer srv.Close()
	c := New("", 5)
	c.viesURL = srv.URL
	res, err := c.Lookup(context.Background(), Query{VATNumber: "FR00000000000"})
	if err != nil {
		t.Fatal(err)
	}
	if *res.Companies[0].VATValid != false {
		t.Fatalf("invalid VAT must be reported as invalid: %+v", res.Companies[0])
	}
}

func TestFranceLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "boulangerie dupont" {
			t.Errorf("query not passed through: %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, `{"total_results":1,"results":[{"nom_complet":"BOULANGERIE DUPONT","siren":"123456789","etat_administratif":"A","nature_juridique":"SARL","date_creation":"2010-01-01","siege":{"adresse":"1 RUE DE PARIS 75001 PARIS"}}]}`)
	}))
	defer srv.Close()
	c := New("", 5)
	c.frURL = srv.URL
	res, err := c.Lookup(context.Background(), Query{Name: "boulangerie dupont", Country: "FR"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != "recherche-entreprises" || res.Total != 1 || len(res.Companies) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	co := res.Companies[0]
	if co.Number != "123456789" || co.Status != "active" || co.Address != "1 RUE DE PARIS 75001 PARIS" {
		t.Fatalf("unexpected company: %+v", co)
	}
}

func TestRomaniaLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if len(body) != 1 || body[0]["cui"] != float64(12345) {
			t.Errorf("unexpected ANAF body: %v", body)
		}
		fmt.Fprint(w, `{"cod":200,"found":[{"denumire":"ACME SRL","cui":12345,"adresa":"BUCURESTI, STR. UNIRII 1","statusInactivi":false,"scpTVA":true}]}`)
	}))
	defer srv.Close()
	c := New("", 5)
	c.roURL = srv.URL
	res, err := c.Lookup(context.Background(), Query{FiscalCode: "RO12345", Country: "RO"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != "anaf" || len(res.Companies) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	co := res.Companies[0]
	if co.Name != "ACME SRL" || co.Status != "active" || co.Type != "VAT-registered" {
		t.Fatalf("unexpected company: %+v", co)
	}
}

func TestRomaniaRejectsBadCUI(t *testing.T) {
	c := New("", 5)
	if _, err := c.Lookup(context.Background(), Query{FiscalCode: "abc", Country: "RO"}); err == nil {
		t.Fatal("expected invalid CUI to be rejected")
	}
}

func TestCompaniesHouseLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "k" || p != "" {
			t.Errorf("expected basic auth with key as username")
		}
		fmt.Fprint(w, `{"total_results":1,"items":[{"title":"ACME LIMITED","company_number":"12345678","company_status":"active","company_type":"ltd","date_of_creation":"2010-01-01","address_snippet":"1 High Street, London"}]}`)
	}))
	defer srv.Close()
	c := New("k", 5)
	c.chURL = srv.URL
	res, err := c.Lookup(context.Background(), Query{Name: "Acme", Country: "GB"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != "companies_house" || len(res.Companies) != 1 || res.Companies[0].Number != "12345678" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestProviderHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()
	c := New("", 5)
	c.viesURL = srv.URL
	if _, err := c.Lookup(context.Background(), Query{VATNumber: "DE123456789"}); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected status error, got %v", err)
	}
}
