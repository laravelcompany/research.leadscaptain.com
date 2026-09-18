package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"research-leads/internal/companies"
)

// CompanySearch merges autocomplete, registries and the cache for the
// Companies search bar. GET /api/v1/companies/search?q=&country=&industry=&location=
func (s *Server) CompanySearch(w http.ResponseWriter, r *http.Request) {
	if s.Companies == nil {
		writeError(w, http.StatusServiceUnavailable, "companies service unavailable")
		return
	}
	q := companies.SearchQuery{
		Query:    r.URL.Query().Get("q"),
		Country:  r.URL.Query().Get("country"),
		Industry: r.URL.Query().Get("industry"),
		Location: r.URL.Query().Get("location"),
	}
	if strings.TrimSpace(q.Query) == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.Companies.Search(r.Context(), q))
}

// CompanySuggest feeds the search box type-ahead. GET /api/v1/companies/suggest?q=
func (s *Server) CompanySuggest(w http.ResponseWriter, r *http.Request) {
	if s.Companies == nil || s.Companies.Autocomplete == nil {
		writeError(w, http.StatusServiceUnavailable, "companies service unavailable")
		return
	}
	sug, err := s.Companies.Autocomplete.Suggest(r.Context(), r.URL.Query().Get("q"), 8)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sug == nil {
		sug = []companies.Suggestion{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"suggestions": sug})
}

// CompanyProfile assembles the full company view. GET /api/v1/companies/profile?domain=&name=&country=
func (s *Server) CompanyProfile(w http.ResponseWriter, r *http.Request) {
	if s.Companies == nil {
		writeError(w, http.StatusServiceUnavailable, "companies service unavailable")
		return
	}
	q := r.URL.Query()
	p, err := s.Companies.Profile(r.Context(), q.Get("domain"), q.Get("name"), q.Get("country"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// CompaniesList lists cached company profiles with search + pagination.
func (s *Server) CompaniesList(w http.ResponseWriter, r *http.Request) {
	if s.Companies == nil || s.Companies.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "companies service unavailable")
		return
	}
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	per, _ := strconv.Atoi(q.Get("per_page"))
	if per < 1 || per > 100 {
		per = 25
	}
	where, args := "1=1", []any{}
	if v := strings.TrimSpace(q.Get("search")); v != "" {
		where = "(name LIKE ? OR domain LIKE ? OR industry LIKE ?)"
		like := "%" + v + "%"
		args = append(args, like, like, like)
	}
	var total int
	if err := s.DB.QueryRow("SELECT count(*) FROM companies WHERE "+where, args...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := s.DB.Query(`SELECT id,name,domain,industry,employee_count,employee_range,founded_year,hq_location,revenue_estimate,description,linkedin_url,twitter_url,facebook_url,crunchbase_url,tech_stack,source,registry_number,registry_status,registry_country,created_at,updated_at FROM companies WHERE `+where+" ORDER BY updated_at DESC LIMIT ? OFFSET ?", append(args, per, (page-1)*per)...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var v [20]sql.NullString
		if err := rows.Scan(&id, &v[0], &v[1], &v[2], &v[3], &v[4], &v[5], &v[6], &v[7], &v[8], &v[9], &v[10], &v[11], &v[12], &v[13], &v[14], &v[15], &v[16], &v[17], &v[18], &v[19]); err != nil {
			continue
		}
		keys := []string{"name", "domain", "industry", "employee_count", "employee_range", "founded_year", "hq_location", "revenue_estimate", "description", "linkedin_url", "twitter_url", "facebook_url", "crunchbase_url", "tech_stack", "source", "registry_number", "registry_status", "registry_country", "created_at", "updated_at"}
		m := map[string]any{"id": id}
		for i, k := range keys {
			m[k] = v[i].String
		}
		out = append(out, m)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": out, "page": page, "per_page": per, "total": total, "last_page": max(1, (total+per-1)/per)})
}

// CompaniesExport streams the cached companies as CSV.
func (s *Server) CompaniesExport(w http.ResponseWriter, r *http.Request) {
	if s.Companies == nil || s.Companies.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "companies service unavailable")
		return
	}
	rows, err := s.DB.Query("SELECT name,domain,industry,employee_count,employee_range,founded_year,hq_location,revenue_estimate,description,linkedin_url,twitter_url,facebook_url,crunchbase_url,tech_stack,source,registry_number,registry_status,registry_country,created_at,updated_at FROM companies ORDER BY id")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="companies.csv"`)
	cw := csv.NewWriter(w)
	cw.Write([]string{"name", "domain", "industry", "employee_count", "employee_range", "founded_year", "hq_location", "revenue_estimate", "description", "linkedin_url", "twitter_url", "facebook_url", "crunchbase_url", "tech_stack", "source", "registry_number", "registry_status", "registry_country", "created_at", "updated_at"})
	defer rows.Close()
	for rows.Next() {
		var v [20]sql.NullString
		if err := rows.Scan(&v[0], &v[1], &v[2], &v[3], &v[4], &v[5], &v[6], &v[7], &v[8], &v[9], &v[10], &v[11], &v[12], &v[13], &v[14], &v[15], &v[16], &v[17], &v[18], &v[19]); err != nil {
			continue
		}
		row := make([]string, len(v))
		for i := range v {
			row[i] = v[i].String
		}
		cw.Write(row)
	}
	cw.Flush()
}

// CompanySaveToLead converts a company result into a lead.
// POST /api/v1/companies/save-to-lead {"company_id":N} or {"name":...,"domain":...}
func (s *Server) CompanySaveToLead(w http.ResponseWriter, r *http.Request) {
	if s.Companies == nil {
		writeError(w, http.StatusServiceUnavailable, "companies service unavailable")
		return
	}
	var body struct {
		CompanyID int64  `json:"company_id"`
		Name      string `json:"name"`
		Domain    string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	id, created, err := s.Companies.SaveToLead(r.Context(), body.CompanyID, body.Name, body.Domain)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"lead_id": id, "created": created})
}
