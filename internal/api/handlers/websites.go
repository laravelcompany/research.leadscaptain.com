package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"research-leads/internal/webcheck"
)

// AnalyzeWebsite runs the SEO/contact analysis for one URL and stores the
// report. POST /api/v1/websites/analyze {"url":"example.com"}
func (s *Server) AnalyzeWebsite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(body.URL) == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	analyze := s.AnalyzeSite
	if analyze == nil {
		analyze = webcheck.Analyze
	}
	a := analyze(r.Context(), body.URL)
	if s.DB != nil {
		h1, _ := json.Marshal(a.H1Tags)
		checks, _ := json.Marshal(a.Checks)
		contacts, _ := json.Marshal(map[string]any{"emails": a.Emails, "phones": a.Phones, "socials": a.Socials})
		kw, _ := json.Marshal(a.Keywords)
		reachable := 0
		if a.Reachable {
			reachable = 1
		}
		res, err := s.DB.ExecContext(r.Context(), `INSERT INTO website_analyses(url,domain,reachable,status_code,load_ms,title,meta_description,h1_tags,health_score,seo_checks,contacts,keywords,traffic_note,error)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			a.URL, a.Domain, reachable, a.StatusCode, a.LoadMs, a.Title, a.MetaDescription, string(h1), a.HealthScore, string(checks), string(contacts), string(kw), a.TrafficNote, a.Error)
		if err == nil {
			if id, err := res.LastInsertId(); err == nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"id": id, "analysis": a})
				return
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"analysis": a})
}

// WebsitesList returns past analyses, newest first, with search + pagination.
func (s *Server) WebsitesList(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
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
		where = "(domain LIKE ? OR url LIKE ?)"
		like := "%" + v + "%"
		args = append(args, like, like)
	}
	var total int
	if err := s.DB.QueryRow("SELECT count(*) FROM website_analyses WHERE "+where, args...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := s.DB.Query("SELECT id,url,domain,reachable,status_code,load_ms,title,health_score,error,created_at FROM website_analyses WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", append(args, per, (page-1)*per)...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, reachable, status, load, score int64
		var url, domain, title, errStr, created sql.NullString
		if err := rows.Scan(&id, &url, &domain, &reachable, &status, &load, &title, &score, &errStr, &created); err != nil {
			continue
		}
		out = append(out, map[string]any{
			"id": id, "url": url.String, "domain": domain.String, "reachable": reachable == 1,
			"status_code": status, "load_ms": load, "title": title.String,
			"health_score": score, "error": errStr.String, "created_at": created.String,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": out, "page": page, "per_page": per, "total": total, "last_page": max(1, (total+per-1)/per)})
}

// WebsiteByID returns one stored report with the full JSON blobs decoded.
func (s *Server) WebsiteByID(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	id := chi.URLParam(r, "id")
	var (
		url, domain, title, desc, errStr, traffic, created sql.NullString
		h1, checks, contacts, kw                           sql.NullString
		reachable, status, load, score                     sql.NullInt64
	)
	err := s.DB.QueryRow("SELECT url,domain,reachable,status_code,load_ms,title,meta_description,h1_tags,health_score,seo_checks,contacts,keywords,traffic_note,error,created_at FROM website_analyses WHERE id=?", id).
		Scan(&url, &domain, &reachable, &status, &load, &title, &desc, &h1, &score, &checks, &contacts, &kw, &traffic, &errStr, &created)
	if err != nil {
		writeError(w, http.StatusNotFound, "analysis not found")
		return
	}
	var h1v, checksv, contactsv, kwv any
	json.Unmarshal([]byte(h1.String), &h1v)
	json.Unmarshal([]byte(checks.String), &checksv)
	json.Unmarshal([]byte(contacts.String), &contactsv)
	json.Unmarshal([]byte(kw.String), &kwv)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id": id, "url": url.String, "domain": domain.String, "reachable": reachable.Int64 == 1,
		"status_code": status.Int64, "load_ms": load.Int64, "title": title.String, "meta_description": desc.String,
		"h1_tags": h1v, "health_score": score.Int64, "checks": checksv, "contacts": contactsv,
		"keywords": kwv, "traffic_note": traffic.String, "error": errStr.String, "created_at": created.String,
	})
}
