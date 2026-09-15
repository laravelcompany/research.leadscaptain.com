package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"research-leads/internal/companyreg"
	"research-leads/internal/emailvalidator"
	"research-leads/internal/webcheck"
)

func TestFindEmailPatterns(t *testing.T) {
	got := Patterns("Ada", "Lovelace", "acme.com")
	if len(got) == 0 || got[0] != "ada.lovelace@acme.com" {
		t.Fatalf("first.last must be the primary pattern, got %v", got)
	}
	joined := strings.Join(got, ",")
	for _, want := range []string{"ada@acme.com", "alovelace@acme.com", "adalovelace@acme.com"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing pattern %s in %v", want, got)
		}
	}
	if p := Patterns("", "Lovelace", "acme.com"); p != nil {
		t.Fatalf("no first name must yield no patterns, got %v", p)
	}
	if p := Patterns("Ada", "Lovelace", "notadomain"); p != nil {
		t.Fatalf("invalid domain must yield no patterns, got %v", p)
	}
}

func TestFindEmailVerifiesCandidates(t *testing.T) {
	ev := emailvalidator.New("", "", 5) // deterministic mock
	tool := NewFindEmail(ev)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"first_name":"Ada","last_name":"Lovelace","domain":"acme.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	cands := m["candidates"].([]Candidate)
	if len(cands) == 0 {
		t.Fatal("expected candidates")
	}
	for _, c := range cands {
		if c.Status == "" {
			t.Fatalf("candidate %s has empty status", c.Email)
		}
	}
}

func TestCheckDomain(t *testing.T) {
	tool := &CheckDomainTool{
		lookupMX: func(d string) ([]*net.MX, error) {
			return []*net.MX{{Host: "aspmx.l.google.com.", Pref: 1}}, nil
		},
		lookupTXT: func(d string) ([]string, error) {
			if strings.HasPrefix(d, "_dmarc.") {
				return []string{"v=DMARC1; p=reject"}, nil
			}
			return []string{"v=spf1 include:_spf.google.com ~all"}, nil
		},
	}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"domain":"acme.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["has_mx"] != true || m["has_spf"] != true || m["has_dmarc"] != true {
		t.Fatalf("unexpected posture: %v", m)
	}
	if m["mx_provider"] != "google-workspace" {
		t.Fatalf("expected google-workspace provider guess, got %v", m["mx_provider"])
	}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"domain":"bad domain/x"}`)); err == nil {
		t.Fatal("expected invalid domain to be rejected")
	}
}

func TestCheckDomainNoMX(t *testing.T) {
	tool := &CheckDomainTool{
		lookupMX:  func(d string) ([]*net.MX, error) { return nil, errors.New("no such host") },
		lookupTXT: func(d string) ([]string, error) { return nil, errors.New("no such host") },
	}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"domain":"dead.example"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["has_mx"] != false || m["has_spf"] != false || m["has_dmarc"] != false {
		t.Fatalf("dead domain must show no email posture: %v", m)
	}
}

func TestCheckWebsite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "cloudflare")
		w.Write([]byte(`<html><head><title>Acme Ltd - We make things</title>
<meta name="description" content="Acme builds anvils.">
<meta name="generator" content="WordPress 6.5"></head>
<body><script src="/wp-content/themes/acme.js"></script><script src="https://js.stripe.com/v3/"></script></body></html>`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	out, err := NewCheckWebsite().Execute(context.Background(), json.RawMessage(`{"domain":"`+host+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	res := out.(webcheck.Result)
	if !res.Reachable || res.StatusCode != 200 {
		t.Fatalf("expected reachable 200, got %+v", res)
	}
	if res.Title != "Acme Ltd - We make things" {
		t.Fatalf("bad title: %q", res.Title)
	}
	if res.Description != "Acme builds anvils." {
		t.Fatalf("bad description: %q", res.Description)
	}
	if res.Generator != "WordPress 6.5" {
		t.Fatalf("bad generator: %q", res.Generator)
	}
	tech := strings.Join(res.Technologies, ",")
	for _, want := range []string{"WordPress", "Stripe", "Cloudflare"} {
		if !strings.Contains(tech, want) {
			t.Fatalf("expected %s in technologies %v", want, res.Technologies)
		}
	}
}

func TestCheckWebsiteInvalidDomain(t *testing.T) {
	out, err := NewCheckWebsite().Execute(context.Background(), json.RawMessage(`{"domain":"nodot"}`))
	if err != nil {
		t.Fatal(err)
	}
	res := out.(webcheck.Result)
	if res.Reachable || res.Error == "" {
		t.Fatalf("invalid domain must be unreachable with an error, got %+v", res)
	}
}

func TestCompanyLookupToolRejectsEmptyInput(t *testing.T) {
	tool := NewCompanyLookup(companyreg.New("", 5))
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected empty input to be rejected")
	}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"name":"Acme","country":"DE"}`)); err == nil {
		t.Fatal("expected unsupported name-search country to fail safe")
	}
}
