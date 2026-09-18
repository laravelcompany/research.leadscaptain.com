package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"research-leads/internal/companies"
)

func TestCompanySaveToLeadHandler(t *testing.T) {
	d := objectiveTestDB(t)
	s := &Server{DB: d, Companies: &companies.Service{DB: d}}
	req := httptest.NewRequest("POST", "/api/v1/companies/save-to-lead", strings.NewReader(`{"name":"Acme","domain":"acme.com"}`))
	rec := httptest.NewRecorder()
	s.CompanySaveToLead(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["created"] != true {
		t.Fatalf("expected created=true, got %v", resp)
	}
	var company, domain, src string
	d.QueryRow("SELECT company_name,company_domain,source FROM leads WHERE company_domain='acme.com'").Scan(&company, &domain, &src)
	if company != "Acme" || src != "company_search" {
		t.Fatalf("lead row wrong: %q %q %q", company, domain, src)
	}
}

func TestCompanySaveToLeadValidation(t *testing.T) {
	d := objectiveTestDB(t)
	s := &Server{DB: d, Companies: &companies.Service{DB: d}}
	req := httptest.NewRequest("POST", "/api/v1/companies/save-to-lead", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	s.CompanySaveToLead(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCompaniesListPagination(t *testing.T) {
	d := objectiveTestDB(t)
	d.Exec("INSERT INTO companies(name,domain) VALUES('Acme','acme.com'),('Globex','globex.com')")
	s := &Server{DB: d, Companies: &companies.Service{DB: d}}
	rec := httptest.NewRecorder()
	s.CompaniesList(rec, httptest.NewRequest("GET", "/api/v1/companies?search=acme", nil))
	if rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	var resp struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Total != 1 || len(resp.Data) != 1 || resp.Data[0]["name"] != "Acme" {
		t.Fatalf("unexpected list response: %+v", resp)
	}
}

func TestCompaniesExportCSV(t *testing.T) {
	d := objectiveTestDB(t)
	d.Exec("INSERT INTO companies(name,domain,industry) VALUES('Acme','acme.com','Software')")
	s := &Server{DB: d, Companies: &companies.Service{DB: d}}
	rec := httptest.NewRecorder()
	s.CompaniesExport(rec, httptest.NewRequest("GET", "/api/v1/companies/export", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("content type %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Acme") || !strings.Contains(body, "registry_country") {
		t.Fatalf("csv missing expected content:\n%s", body)
	}
}

func TestCompanySearchRequiresQuery(t *testing.T) {
	d := objectiveTestDB(t)
	s := &Server{DB: d, Companies: &companies.Service{DB: d}}
	rec := httptest.NewRecorder()
	s.CompanySearch(rec, httptest.NewRequest("GET", "/api/v1/companies/search", nil))
	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
