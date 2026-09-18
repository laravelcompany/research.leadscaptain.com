package companies

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"research-leads/internal/companyreg"
	"research-leads/internal/db"
	"research-leads/internal/webcheck"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestSuggestParsesClearbitAutocomplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "acme" {
			t.Errorf("unexpected query %q", r.URL.Query().Get("query"))
		}
		json.NewEncoder(w).Encode([]map[string]string{
			{"name": "Acme Corp", "domain": "acme.com", "logo": "https://logo/acme"},
			{"name": "Acme France", "domain": "acme.fr"},
		})
	}))
	defer srv.Close()
	c := NewAutocomplete(5)
	c.baseURL = srv.URL
	got, err := c.Suggest(context.Background(), "acme", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "Acme Corp" || got[0].Domain != "acme.com" {
		t.Fatalf("unexpected suggestions: %+v", got)
	}
}

func TestSuggestIsSoftWhenServiceDown(t *testing.T) {
	c := NewAutocomplete(1)
	c.baseURL = "http://127.0.0.1:1" // nothing listening
	got, err := c.Suggest(context.Background(), "acme", 5)
	if err != nil {
		t.Fatalf("autocomplete failure must be soft, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no suggestions, got %+v", got)
	}
}

func TestSuggestSkipsShortQueries(t *testing.T) {
	c := NewAutocomplete(1)
	if got, _ := c.Suggest(context.Background(), "a", 5); got != nil {
		t.Fatalf("expected nil for short query, got %+v", got)
	}
}

func TestSearchMergesAndDedupes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]string{{"name": "Acme", "domain": "acme.com"}})
	}))
	defer srv.Close()
	ac := NewAutocomplete(5)
	ac.baseURL = srv.URL
	d := testDB(t)
	d.Exec("INSERT INTO companies(name,domain) VALUES('Acme','acme.com'),('Acme Cached','acme-cached.example')")
	s := &Service{DB: d, Autocomplete: ac} // no registry: FR/GB skipped
	resp := s.Search(context.Background(), SearchQuery{Query: "Acme"})
	if len(resp.Results) != 2 {
		t.Fatalf("expected autocomplete+cache deduped to 2, got %+v", resp.Results)
	}
	if resp.Results[0].Domain != "acme.com" {
		t.Fatalf("autocomplete result should win the dedupe, got %+v", resp.Results[0])
	}
}

func TestSearchRequiresQueryForNotesOnly(t *testing.T) {
	s := &Service{Registry: companyreg.New("", 5)}
	resp := s.Search(context.Background(), SearchQuery{Query: "acme", Country: "DE"})
	if len(resp.Results) != 0 {
		t.Fatalf("expected no results without sources, got %+v", resp.Results)
	}
	found := false
	for _, n := range resp.Notes {
		if n != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a note explaining DE name search is unsupported")
	}
}

func TestProfileAssemblesWebsiteSignals(t *testing.T) {
	d := testDB(t)
	s := &Service{
		DB: d,
		CheckSite: func(ctx context.Context, domain string) webcheck.Result {
			return webcheck.Result{
				Domain: domain, Reachable: true, Title: "Acme - Industrial widgets",
				Description: "Acme makes widgets.", Technologies: []string{"Shopify", "React"},
				Socials: webcheck.SocialLinks{LinkedIn: "https://linkedin.com/company/acme", Twitter: "https://x.com/acme"},
			}
		},
	}
	p, err := s.Profile(context.Background(), "acme.com", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Acme - Industrial widgets" || p.Description != "Acme makes widgets." {
		t.Fatalf("profile missing website fields: %+v", p)
	}
	if len(p.TechStack) != 2 || p.LinkedIn == "" {
		t.Fatalf("profile missing tech/socials: %+v", p)
	}
	// Without a Clearbit key the profile must say why employees/revenue are blank.
	noted := false
	for _, n := range p.Notes {
		if n == "employee count and revenue need CLEARBIT_API_KEY; not available from free sources" {
			noted = true
		}
	}
	if !noted {
		t.Fatalf("expected no-key note, got %+v", p.Notes)
	}
	// The profile must be cached for repeat views.
	var count int
	d.QueryRow("SELECT count(*) FROM companies WHERE domain='acme.com'").Scan(&count)
	if count != 1 {
		t.Fatalf("expected cached company row, got %d", count)
	}
}

func TestProfileRequiresInput(t *testing.T) {
	s := &Service{}
	if _, err := s.Profile(context.Background(), "", "", ""); err == nil {
		t.Fatal("expected error when neither domain nor name given")
	}
}

func TestSaveToLeadDedupesOnDomain(t *testing.T) {
	d := testDB(t)
	s := &Service{DB: d}
	id1, created1, err := s.SaveToLead(context.Background(), 0, "Acme", "acme.com")
	if err != nil || !created1 || id1 == 0 {
		t.Fatalf("first save: id=%d created=%v err=%v", id1, created1, err)
	}
	id2, created2, err := s.SaveToLead(context.Background(), 0, "Acme", "acme.com")
	if err != nil || created2 || id2 != id1 {
		t.Fatalf("second save must dedupe: id=%d created=%v err=%v", id2, created2, err)
	}
	var src string
	d.QueryRow("SELECT source FROM leads WHERE id=?", id1).Scan(&src)
	if src != "company_search" {
		t.Fatalf("expected source=company_search, got %q", src)
	}
}

func TestClearbitOverlayFillsGapsOnly(t *testing.T) {
	p := Profile{Name: "Registry Name", LinkedIn: "https://linkedin.com/company/acme"}
	overlayProfile(&p, &Enrichment{Name: "Clearbit Name", Industry: "Software", LinkedIn: "https://linkedin.com/company/other"})
	if p.Name != "Registry Name" {
		t.Fatalf("registry name must win, got %q", p.Name)
	}
	if p.Industry != "Software" || p.LinkedIn != "https://linkedin.com/company/acme" {
		t.Fatalf("overlay misapplied: %+v", p)
	}
}
