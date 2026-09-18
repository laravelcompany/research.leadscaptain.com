package researchtools

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func mxOK(hosts ...string) MXLookup {
	return func(ctx context.Context, domain string) ([]*net.MX, error) {
		var out []*net.MX
		for _, h := range hosts {
			out = append(out, &net.MX{Host: h})
		}
		return out, nil
	}
}

func TestVerifyEmailValidGmail(t *testing.T) {
	v := VerifyEmail(context.Background(), "Jane.Doe@gmail.com", mxOK("aspmx.l.google.com."))
	if v.Status != "valid" || !v.MXFound || v.Provider != "Gmail / Google Workspace" {
		t.Fatalf("%+v", v)
	}
	if v.Email != "jane.doe@gmail.com" {
		t.Fatalf("email should be normalized: %q", v.Email)
	}
}

func TestVerifyEmailInvalidSyntax(t *testing.T) {
	for _, bad := range []string{"not-an-email", "a@", "@b.com", "a@b"} {
		if v := VerifyEmail(context.Background(), bad, mxOK("mx.example.com.")); v.Status != "invalid" {
			t.Fatalf("%s should be invalid: %+v", bad, v)
		}
	}
}

func TestVerifyEmailDisposableIsRisky(t *testing.T) {
	v := VerifyEmail(context.Background(), "x@mailinator.com", mxOK("mx.mailinator.com."))
	if v.Status != "risky" {
		t.Fatalf("%+v", v)
	}
}

func TestVerifyEmailRoleAddressIsRisky(t *testing.T) {
	v := VerifyEmail(context.Background(), "info@acme.com", mxOK("mx.acme.com."))
	if v.Status != "risky" || v.Reason == "" {
		t.Fatalf("%+v", v)
	}
}

func TestVerifyEmailNoMXNoHostIsInvalid(t *testing.T) {
	lookup := func(ctx context.Context, domain string) ([]*net.MX, error) {
		return nil, errors.New("no such host")
	}
	v := VerifyEmail(context.Background(), "a@this-domain-should-not-exist-xyz123.invalid", lookup)
	if v.Status != "invalid" {
		t.Fatalf("%+v", v)
	}
}

func TestDomainAgeParsesRDAP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"events": []map[string]string{
				{"eventAction": "registration", "eventDate": "2015-06-15T10:00:00Z"},
				{"eventAction": "expiration", "eventDate": "2027-06-15T10:00:00Z"},
			},
			"entities": []map[string]any{
				{"roles": []string{"registrar"}, "vcardArray": []any{"vcard", []any{[]any{"version", map[string]any{}, "text", "4.0"}, []any{"fn", map[string]any{}, "text", "Example Registrar LLC"}}}},
			},
		})
	}))
	defer srv.Close()
	c := NewDomainAgeChecker(5)
	c.baseURL = srv.URL + "/"
	a := c.Check(context.Background(), "example.com")
	if a.Error != "" {
		t.Fatal(a.Error)
	}
	if a.Created != "2015-06-15" || a.Expires != "2027-06-15" || a.Registrar != "Example Registrar LLC" {
		t.Fatalf("%+v", a)
	}
	if a.AgeDisplay == "" {
		t.Fatal("expected age display")
	}
}

func TestDomainAgeNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer srv.Close()
	c := NewDomainAgeChecker(5)
	c.baseURL = srv.URL + "/"
	if a := c.Check(context.Background(), "nope.example"); a.Error == "" {
		t.Fatal("expected error")
	}
}

func TestHumanAge(t *testing.T) {
	created := time.Now().AddDate(-8, -3, 0).Format("2006-01-02")
	if got := humanAge(created); got != "8 years 3 months" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatLinkedIn(t *testing.T) {
	cases := []struct{ in, want, kind string }{
		{"https://www.linkedin.com/in/jane-doe/", "https://www.linkedin.com/in/jane-doe", "profile"},
		{"linkedin.com/in/jane-doe?utm_source=x", "https://www.linkedin.com/in/jane-doe", "profile"},
		{"uk.linkedin.com/in/jane-doe", "https://www.linkedin.com/in/jane-doe", "profile"},
		{"in/jane-doe", "https://www.linkedin.com/in/jane-doe", "profile"},
		{"jane-doe", "https://www.linkedin.com/in/jane-doe", "profile"},
		{"https://www.linkedin.com/company/acme-ltd", "https://www.linkedin.com/company/acme-ltd", "company"},
	}
	for _, c := range cases {
		got := FormatLinkedIn(c.in)
		if got.Formatted != c.want || got.Kind != c.kind || got.Error != "" {
			t.Fatalf("%s: %+v", c.in, got)
		}
	}
	for _, bad := range []string{"", "https://linkedin.com/posts/123", "https://example.com/in/x", "in/not ok!"} {
		if got := FormatLinkedIn(bad); got.Error == "" {
			t.Fatalf("%q should fail: %+v", bad, got)
		}
	}
}

func TestBulkJobProcessesAndCompletes(t *testing.T) {
	s := &Service{
		DB: testDBTools(t),
		Verify: func(ctx context.Context, email string) EmailVerdict {
			return EmailVerdict{Email: email, Status: "valid"}
		},
	}
	id, err := s.StartBulk(context.Background(), "emails", []string{"a@x.com", "b@y.com", ""})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		j, err := s.BulkStatus(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if j.Status == "completed" {
			if j.Completed != 2 || j.Total != 2 {
				t.Fatalf("%+v", j)
			}
			var results []EmailVerdict
			if err := json.Unmarshal(j.Results, &results); err != nil || len(results) != 2 {
				t.Fatalf("results: %v %s", err, j.Results)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBulkValidation(t *testing.T) {
	s := &Service{DB: testDBTools(t)}
	if _, err := s.StartBulk(context.Background(), "bogus", []string{"a"}); err == nil {
		t.Fatal("bad type accepted")
	}
	if _, err := s.StartBulk(context.Background(), "emails", nil); err == nil {
		t.Fatal("empty batch accepted")
	}
	items := make([]string, MaxBulkItems+1)
	for i := range items {
		items[i] = "x"
	}
	if _, err := s.StartBulk(context.Background(), "emails", items); err == nil {
		t.Fatal("oversized batch accepted")
	}
}
