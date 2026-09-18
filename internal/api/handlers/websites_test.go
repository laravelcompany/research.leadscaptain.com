package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"research-leads/internal/webcheck"
)

func TestAnalyzeWebsiteStoresReport(t *testing.T) {
	d := objectiveTestDB(t)
	s := &Server{DB: d, AnalyzeSite: func(ctx context.Context, rawURL string) webcheck.Analysis {
		return webcheck.Analysis{URL: "https://acme.example", Domain: "acme.example", Reachable: true, StatusCode: 200, LoadMs: 120, Title: "Acme", HealthScore: 85, TrafficNote: "unavailable"}
	}}
	req := httptest.NewRequest("POST", "/api/v1/websites/analyze", strings.NewReader(`{"url":"acme.example"}`))
	rec := httptest.NewRecorder()
	s.AnalyzeWebsite(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID       int64             `json:"id"`
		Analysis webcheck.Analysis `json:"analysis"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID == 0 || resp.Analysis.HealthScore != 85 {
		t.Fatalf("unexpected response %+v", resp)
	}
	var count int
	d.QueryRow("SELECT count(*) FROM website_analyses WHERE domain='acme.example'").Scan(&count)
	if count != 1 {
		t.Fatalf("expected stored report, got %d", count)
	}
}

func TestAnalyzeWebsiteValidation(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	s.AnalyzeWebsite(rec, httptest.NewRequest("POST", "/api/v1/websites/analyze", strings.NewReader(`{}`)))
	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestWebsitesListAndGet(t *testing.T) {
	d := objectiveTestDB(t)
	d.Exec(`INSERT INTO website_analyses(url,domain,reachable,status_code,load_ms,title,health_score,seo_checks,contacts,keywords,h1_tags) VALUES('https://acme.example','acme.example',1,200,100,'Acme',90,'[]','{}','[]','[]')`)
	s := &Server{DB: d}
	rec := httptest.NewRecorder()
	s.WebsitesList(rec, httptest.NewRequest("GET", "/api/v1/websites?search=acme", nil))
	if rec.Code != 200 {
		t.Fatalf("list code %d", rec.Code)
	}
	var list struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
	}
	json.NewDecoder(rec.Body).Decode(&list)
	if list.Total != 1 || list.Data[0]["health_score"].(float64) != 90 {
		t.Fatalf("unexpected list %+v", list)
	}

	rec2 := httptest.NewRecorder()
	s.WebsiteByID(rec2, withID(httptest.NewRequest("GET", "/api/v1/websites/1", nil), "1"))
	if rec2.Code != 200 {
		t.Fatalf("get code %d", rec2.Code)
	}
	var one map[string]any
	json.NewDecoder(rec2.Body).Decode(&one)
	if one["domain"] != "acme.example" {
		t.Fatalf("unexpected report %+v", one)
	}
}
