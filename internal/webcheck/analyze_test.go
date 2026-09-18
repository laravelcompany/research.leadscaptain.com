package webcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const homepageHTML = `<html lang="en"><head>
<title>Acme Widgets - Industrial Widgets Ltd</title>
<meta name="description" content="Acme Widgets builds durable industrial widgets for factories across Europe and beyond.">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="canonical" href="https://acme.example/">
</head><body>
<h1>Industrial Widgets</h1>
<p>We build widgets and more widgets. Our widgets team loves widgets, factories and quality engineering.</p>
<a href="mailto:sales@acme.example">Email us</a>
<a href="tel:+442071234567">Call</a>
<a href="https://linkedin.com/company/acme-widgets">LinkedIn</a>
</body></html>`

func analyzeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(homepageHTML))
		case "/contact":
			w.Write([]byte(`<html><body><p>Contact: support@acme.example or +44 20 7946 0958</p></body></html>`))
		default:
			w.WriteHeader(404)
		}
	}))
}

func TestAnalyzeExtractsSEOContactsAndScores(t *testing.T) {
	srv := analyzeServer(t)
	defer srv.Close()
	a := Analyze(context.Background(), srv.URL)
	if !a.Reachable || a.StatusCode != 200 {
		t.Fatalf("expected reachable 200, got %+v", a)
	}
	if a.Title != "Acme Widgets - Industrial Widgets Ltd" {
		t.Fatalf("title %q", a.Title)
	}
	if len(a.H1Tags) != 1 || a.H1Tags[0] != "Industrial Widgets" {
		t.Fatalf("h1 %+v", a.H1Tags)
	}
	foundSales, foundSupport := false, false
	for _, e := range a.Emails {
		if e == "sales@acme.example" {
			foundSales = true
		}
		if e == "support@acme.example" {
			foundSupport = true
		}
	}
	if !foundSales || !foundSupport {
		t.Fatalf("emails should combine homepage + contact page: %+v", a.Emails)
	}
	if len(a.Phones) == 0 {
		t.Fatal("expected at least one phone number")
	}
	if a.Socials.LinkedIn != "https://linkedin.com/company/acme-widgets" {
		t.Fatalf("socials %+v", a.Socials)
	}
	if a.HealthScore <= 0 || a.HealthScore > 100 {
		t.Fatalf("score %d out of range", a.HealthScore)
	}
	maxTotal := 0
	for _, c := range a.Checks {
		maxTotal += c.Max
	}
	if maxTotal != 100 {
		t.Fatalf("checks must sum to 100, got %d", maxTotal)
	}
	if a.TrafficNote == "" {
		t.Fatal("traffic note must explain estimates are unavailable")
	}
}

func TestAnalyzeKeywordDensity(t *testing.T) {
	srv := analyzeServer(t)
	defer srv.Close()
	a := Analyze(context.Background(), srv.URL)
	top := ""
	if len(a.Keywords) > 0 {
		top = a.Keywords[0].Word
	}
	if top != "widgets" {
		t.Fatalf("expected 'widgets' as top keyword, got %+v", a.Keywords)
	}
	for _, k := range a.Keywords {
		if stopwords[k.Word] {
			t.Fatalf("stopword %q leaked into keywords", k.Word)
		}
		if k.Density <= 0 {
			t.Fatalf("density must be positive: %+v", k)
		}
	}
}

func TestAnalyzeInvalidURL(t *testing.T) {
	a := Analyze(context.Background(), "  ")
	if a.Error == "" {
		t.Fatal("expected error for blank url")
	}
}

func TestAnalyzeUnreachable(t *testing.T) {
	a := Analyze(context.Background(), "http://127.0.0.1:1")
	if a.Reachable || a.Error == "" {
		t.Fatalf("expected unreachable with error, got %+v", a)
	}
}

func TestScoreChecksDeterministic(t *testing.T) {
	html := homepageHTML
	a := Analysis{Title: "Acme Widgets - Industrial Widgets Ltd", MetaDescription: strings.Repeat("x", 120), H1Tags: []string{"Industrial Widgets"}, FinalURL: "https://acme.example", LoadMs: 500, Emails: []string{"sales@acme.example"}}
	c1 := scoreChecks(a, html, "https://acme.example")
	total := 0
	for _, c := range c1 {
		total += c.Points
	}
	if total != 100 {
		t.Fatalf("well-formed page should score 100, got %d (%+v)", total, c1)
	}
	a2 := Analysis{LoadMs: 9000}
	total2 := 0
	for _, c := range scoreChecks(a2, "<html><body><h1>a</h1><h1>b</h1></body></html>", "http://x") {
		total2 += c.Points
	}
	if total2 >= total {
		t.Fatalf("bad page should score lower, got %d", total2)
	}
}
