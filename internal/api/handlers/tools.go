package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"research-leads/internal/researchtools"
)

// ToolVerifyEmail: POST /api/v1/tools/verify-email {"email":"a@b.com"}
func (s *Server) ToolVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(body.Email) == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	verify := func() researchtools.EmailVerdict {
		if s.Tools != nil {
			return s.Tools.VerifyEmail(r.Context(), body.Email)
		}
		return researchtools.VerifyEmail(r.Context(), body.Email, nil)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(verify())
}

// ToolDomainAge: GET /api/v1/tools/domain-age?domain=example.com
func (s *Server) ToolDomainAge(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if strings.TrimSpace(domain) == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	if s.Tools == nil || s.Tools.Age == nil {
		writeError(w, http.StatusServiceUnavailable, "domain age checker unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.Tools.Age.Check(r.Context(), domain))
}

// ToolLinkedInFormat: POST /api/v1/tools/linkedin-format {"url":"..."}
func (s *Server) ToolLinkedInFormat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	out := researchtools.FormatLinkedIn(body.URL)
	if out.Error != "" {
		writeError(w, http.StatusBadRequest, out.Error)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// ToolBulkStart: POST /api/v1/tools/bulk {"type":"emails"|"urls","items":[...]}
func (s *Server) ToolBulkStart(w http.ResponseWriter, r *http.Request) {
	if s.Tools == nil {
		writeError(w, http.StatusServiceUnavailable, "tools service unavailable")
		return
	}
	var body struct {
		Type  string   `json:"type"`
		Items []string `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	id, err := s.Tools.StartBulk(r.Context(), body.Type, body.Items)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "queued"})
}

// ToolBulkStatus: GET /api/v1/tools/bulk/{id}
func (s *Server) ToolBulkStatus(w http.ResponseWriter, r *http.Request) {
	if s.Tools == nil {
		writeError(w, http.StatusServiceUnavailable, "tools service unavailable")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	job, err := s.Tools.BulkStatus(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}
