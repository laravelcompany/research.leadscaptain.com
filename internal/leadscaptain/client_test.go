package leadscaptain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchUsesDocumentedLeadsCaptainContract(t *testing.T) {
	var gotPage int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/leads" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "secret" {
			t.Errorf("X-API-Key = %q", got)
		}
		q := r.URL.Query()
		for key, want := range map[string]string{
			"q": "CTO", "position_title": "CTO", "country_code": "GB",
			"location": "London", "industry_name": "Software", "per_page": "25", "page": "3",
		} {
			if got := q.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		gotPage++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"key": "lead-1", "first_name": "Ada", "last_name": "Lovelace",
				"emails": []string{"ada@example.com"}, "company_name": "Analytical",
				"position_title": "CTO", "position_location": "London",
			}},
			"page": 3, "limit": 25, "total": 1, "total_pages": 1,
		})
	}))
	defer srv.Close()

	client := New(srv.URL+"/", "secret", 5)
	leads, err := client.Search(context.Background(), SearchParams{
		Q: "CTO", CountryCode: "GB", City: "London", Industry: "Software", Page: 3, PerPage: 25,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if gotPage != 1 || len(leads) != 1 {
		t.Fatalf("requests = %d, leads = %d", gotPage, len(leads))
	}
	if leads[0].ID != "lead-1" || leads[0].Email != "ada@example.com" || leads[0].City != "London" {
		t.Fatalf("lead mapping = %#v", leads[0])
	}
}

func TestSearchRequiresAPIConfiguration(t *testing.T) {
	for name, client := range map[string]*Client{
		"base URL": New("", "secret", 5),
		"token":    New("https://api.leadscaptain.com", "", 5),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := client.Search(context.Background(), SearchParams{}); err == nil {
				t.Fatal("Search() expected configuration error")
			}
		})
	}
}

func TestSearchIncludesBoundedUpstreamErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream database unavailable", http.StatusBadGateway)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "secret", 5).Search(context.Background(), SearchParams{PerPage: 20})
	if err == nil || !strings.Contains(err.Error(), "leadscaptain 502: upstream database unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}
