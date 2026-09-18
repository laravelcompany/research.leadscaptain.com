package handlers

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"research-leads/internal/agent"
	"research-leads/internal/events"
	"research-leads/internal/leads"
	"research-leads/internal/researchtools"
)

type Server struct {
	DB     *sql.DB
	Bus    *events.Bus
	Engine *agent.Engine
	// VerifyEmail verifies one address and returns its status (e.g. "valid",
	// "invalid", "unknown"). May be nil when no validator is configured.
	VerifyEmail func(ctx context.Context, email string) (string, error)
	// Auth carries the UI login configuration (env-supplied credentials).
	Auth *AuthConfig
	// Tools powers the Tools page (email verify, domain age, bulk jobs).
	Tools *researchtools.Service
}

func (s *Server) Objectives(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		rows, err := s.DB.Query(`SELECT o.id,o.name,o.description,o.status,o.target_leads,o.minimum_score,o.iteration_count,
			COUNT(l.id), COALESCE(SUM(CASE WHEN l.lead_score>=o.minimum_score THEN 1 ELSE 0 END),0)
			FROM objectives o LEFT JOIN leads l ON l.objective_id=o.id
			GROUP BY o.id ORDER BY o.id DESC`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id int64
			var name, description, status string
			var target sql.NullInt64
			var minimum, iterations, discovered, qualified int
			if err := rows.Scan(&id, &name, &description, &status, &target, &minimum, &iterations, &discovered, &qualified); err != nil {
				continue
			}
			m := map[string]any{"id": id, "name": name, "description": description, "status": status, "minimum_score": minimum, "iteration_count": iterations, "leads_discovered": discovered, "qualified": qualified}
			if target.Valid {
				m["target_leads"] = target.Int64
				if target.Int64 > 0 {
					pct := qualified * 100 / int(target.Int64)
					if pct > 100 {
						pct = 100
					}
					m["progress"] = pct
				}
			}
			out = append(out, m)
		}
		json.NewEncoder(w).Encode(out)
	case "POST":
		var body struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			TargetLeads  *int   `json:"target_leads"`
			MinimumScore int    `json:"minimum_score"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		res, err := s.DB.Exec("INSERT INTO objectives(name,description,status,target_leads,minimum_score) VALUES(?,?,'idle',?,?)", body.Name, body.Description, body.TargetLeads, body.MinimumScore)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		id, _ := res.LastInsertId()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": id, "name": body.Name, "status": "idle"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": message}})
}

func (s *Server) ObjectiveByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		writeError(w, http.StatusNotFound, "objective not found")
		return
	}
	if r.Method == "DELETE" {
		var exists int
		if err := s.DB.QueryRow("SELECT 1 FROM objectives WHERE id=?", id).Scan(&exists); err != nil {
			writeError(w, http.StatusNotFound, "objective not found")
			return
		}
		s.DB.Exec("DELETE FROM iterations WHERE run_id IN (SELECT id FROM agent_runs WHERE objective_id=?)", id)
		s.DB.Exec("DELETE FROM agent_runs WHERE objective_id=?", id)
		s.DB.Exec("DELETE FROM search_history WHERE objective_id=?", id)
		s.DB.Exec("DELETE FROM objectives WHERE id=?", id)
		json.NewEncoder(w).Encode(map[string]any{"deleted": id})
		return
	}
	var name, description, status string
	var target sql.NullInt64
	var minimum, iterations int
	if err := s.DB.QueryRow("SELECT name,description,status,target_leads,minimum_score,iteration_count FROM objectives WHERE id=?", id).Scan(&name, &description, &status, &target, &minimum, &iterations); err != nil {
		writeError(w, http.StatusNotFound, "objective not found")
		return
	}
	var discovered, qualified int
	s.DB.QueryRow("SELECT count(*),COALESCE(SUM(CASE WHEN lead_score>=? THEN 1 ELSE 0 END),0) FROM leads WHERE objective_id=?", minimum, id).Scan(&discovered, &qualified)
	m := map[string]any{"id": id, "name": name, "description": description, "status": status, "minimum_score": minimum, "iteration_count": iterations, "leads_discovered": discovered, "qualified": qualified}
	if target.Valid {
		m["target_leads"] = target.Int64
		if target.Int64 > 0 {
			pct := qualified * 100 / int(target.Int64)
			if pct > 100 {
				pct = 100
			}
			m["progress"] = pct
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

func (s *Server) StartObjective(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	runID, err := s.Engine.Start(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"run_id": runID})
}
func (s *Server) PauseObjective(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.Engine.Pause(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.Bus.Publish(events.Event{Type: "agent.paused", Payload: map[string]any{"objective_id": id}})
	json.NewEncoder(w).Encode(map[string]any{"status": "paused"})
}
func (s *Server) ResumeObjective(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.Engine.Resume(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.Bus.Publish(events.Event{Type: "agent.resumed", Payload: map[string]any{"objective_id": id}})
	json.NewEncoder(w).Encode(map[string]any{"status": "running"})
}
func (s *Server) StopObjective(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.Engine.Stop(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.Bus.Publish(events.Event{Type: "agent.stopped", Payload: map[string]any{"objective_id": id}})
	json.NewEncoder(w).Encode(map[string]any{"status": "stopped"})
}

func (s *Server) Leads(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		s.DB.Exec("DELETE FROM leads")
		s.DB.Exec("DELETE FROM search_history")
		json.NewEncoder(w).Encode(map[string]any{"deleted": true})
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
	where := []string{"1=1"}
	args := []any{}
	if v := q.Get("objective_id"); v != "" {
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "invalid objective_id")
			return
		}
		where = append(where, "objective_id=?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("search")); v != "" {
		where = append(where, "(first_name LIKE ? OR last_name LIKE ? OR full_name LIKE ? OR email LIKE ? OR company_name LIKE ? OR company_domain LIKE ? OR city LIKE ?)")
		like := "%" + v + "%"
		for range 7 {
			args = append(args, like)
		}
	}
	if v := q.Get("country"); v != "" {
		where = append(where, "country_code=?")
		args = append(args, v)
	}
	if v := q.Get("status"); v != "" {
		where = append(where, "status=?")
		args = append(args, v)
	}
	if v := q.Get("source"); v != "" {
		where = append(where, "source=?")
		args = append(args, v)
	}
	if v := q.Get("min_score"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid min_score")
			return
		}
		where = append(where, "lead_score>=?")
		args = append(args, n)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := s.DB.QueryRow("SELECT count(*) FROM leads WHERE "+clause, args...).Scan(&total); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	sortColumns := map[string]string{"created_at": "created_at", "updated_at": "updated_at", "name": "COALESCE(full_name,first_name || ' ' || last_name)", "company": "company_name", "score": "lead_score", "status": "status", "source": "source"}
	sort := sortColumns[q.Get("sort")]
	if sort == "" {
		sort = "created_at"
	}
	dir := "DESC"
	if strings.EqualFold(q.Get("direction"), "asc") {
		dir = "ASC"
	}
	query := `SELECT id,objective_id,external_key,first_name,last_name,full_name,email,email_status,phone,company_name,company_domain,position_title,department,industry_name,country_code,country_name,city,linkedin_url,website_url,lead_score,score_breakdown,source,raw_data,status,summary,outreach_message,created_at,updated_at FROM leads WHERE ` + clause + " ORDER BY " + sort + " " + dir + " LIMIT ? OFFSET ?"
	rows, err := s.DB.Query(query, append(args, per, (page-1)*per)...)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, score int64
		var objective sql.NullInt64
		var vals [26]sql.NullString
		if err := rows.Scan(&id, &objective, &vals[0], &vals[1], &vals[2], &vals[3], &vals[4], &vals[5], &vals[6], &vals[7], &vals[8], &vals[9], &vals[10], &vals[11], &vals[12], &vals[13], &vals[14], &vals[15], &vals[16], &score, &vals[17], &vals[18], &vals[19], &vals[20], &vals[21], &vals[22], &vals[23], &vals[24]); err != nil {
			continue
		}
		keys := []string{"external_key", "first_name", "last_name", "full_name", "email", "email_status", "phone", "company_name", "company_domain", "position_title", "department", "industry_name", "country_code", "country_name", "city", "linkedin_url", "website_url", "score_breakdown", "source", "raw_data", "status", "summary", "outreach_message", "created_at", "updated_at"}
		m := map[string]any{"id": id, "lead_score": score}
		if objective.Valid {
			m["objective_id"] = objective.Int64
		}
		for i, k := range keys {
			m[k] = vals[i].String
		}
		out = append(out, m)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": out, "page": page, "per_page": per, "total": total, "last_page": max(1, (total+per-1)/per)})
}

func (s *Server) Lists(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var b struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		res, _ := s.DB.Exec("INSERT INTO lead_lists(name,description) VALUES(?,?)", b.Name, b.Description)
		id, _ := res.LastInsertId()
		json.NewEncoder(w).Encode(map[string]any{"id": id, "name": b.Name})
		return
	}
	rows, _ := s.DB.Query("SELECT id,name,description FROM lead_lists ORDER BY id DESC")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var n, d sql.NullString
			rows.Scan(&id, &n, &d)
			out = append(out, map[string]any{"id": id, "name": n.String, "description": d.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}

func (s *Server) Stats(w http.ResponseWriter, r *http.Request) {
	var leads, qualified, tasks, runs int
	s.DB.QueryRow("SELECT count(*) FROM leads").Scan(&leads)
	s.DB.QueryRow("SELECT count(*) FROM leads WHERE lead_score>=70").Scan(&qualified)
	s.DB.QueryRow("SELECT count(*) FROM tasks").Scan(&tasks)
	s.DB.QueryRow("SELECT count(*) FROM agent_runs").Scan(&runs)
	json.NewEncoder(w).Encode(map[string]any{"leads": leads, "qualified": qualified, "tasks": tasks, "runs": runs})
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}
func (s *Server) Metrics(w http.ResponseWriter, r *http.Request) {
	var leads int
	s.DB.QueryRow("SELECT count(*) FROM leads").Scan(&leads)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("# HELP leads_total\nleads_total " + strconv.Itoa(leads) + "\n"))
}

func eventObjectiveID(e events.Event) string {
	payload, ok := e.Payload.(map[string]any)
	if !ok {
		return ""
	}
	v, ok := payload["objective_id"]
	if !ok {
		return ""
	}
	return fmt.Sprint(v)
}
func (s *Server) Events(w http.ResponseWriter, r *http.Request) {
	objectiveID := strings.TrimSpace(r.URL.Query().Get("objective_id"))
	if objectiveID != "" {
		if _, err := strconv.ParseInt(objectiveID, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "invalid objective_id")
			return
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	ch := s.Bus.Subscribe()
	defer s.Bus.Unsubscribe(ch)
	// initial hello to unblock client
	w.Write([]byte("retry: 3000\n"))
	w.Write([]byte("data: {\"type\":\"connected\",\"payload\":{}}\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case e := <-ch:
			if objectiveID != "" && eventObjectiveID(e) != objectiveID {
				continue
			}
			b, _ := json.Marshal(e)
			w.Write([]byte("data: " + string(b) + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-ticker.C:
			w.Write([]byte(": ping\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) Audit(w http.ResponseWriter, r *http.Request) {
	rows, _ := s.DB.Query("SELECT id, event_type, message FROM audit_log ORDER BY id DESC LIMIT 50")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var et, msg sql.NullString
			rows.Scan(&id, &et, &msg)
			out = append(out, map[string]any{"id": id, "event_type": et.String, "message": msg.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}

func (s *Server) Tasks(w http.ResponseWriter, r *http.Request) {
	rows, _ := s.DB.Query("SELECT id,title,category,status FROM tasks ORDER BY id DESC LIMIT 50")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var t, c, st string
			rows.Scan(&id, &t, &c, &st)
			out = append(out, map[string]any{"id": id, "title": t, "category": c, "status": st})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}
func (s *Server) LeadsExport(w http.ResponseWriter, r *http.Request) {
	where := ""
	args := []any{}
	filename := "leads.csv"
	if objectiveID := strings.TrimSpace(r.URL.Query().Get("objective_id")); objectiveID != "" {
		if _, err := strconv.ParseInt(objectiveID, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "invalid objective_id")
			return
		}
		where = " WHERE objective_id=?"
		args = append(args, objectiveID)
		filename = "objective-" + objectiveID + "-leads.csv"
	}
	rows, err := s.DB.Query("SELECT first_name,last_name,email,company_name,company_domain,position_title,industry_name,country_code,city,lead_score,linkedin_url,website_url,status,source,created_at,updated_at FROM leads"+where+" ORDER BY id", args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	cw := csv.NewWriter(w)
	cw.Write([]string{"first_name", "last_name", "email", "company", "company_domain", "title", "industry", "country", "city", "score", "linkedin", "website", "status", "source", "created_at", "updated_at"})
	defer rows.Close()
	for rows.Next() {
		var v [16]sql.NullString
		if err := rows.Scan(&v[0], &v[1], &v[2], &v[3], &v[4], &v[5], &v[6], &v[7], &v[8], &v[9], &v[10], &v[11], &v[12], &v[13], &v[14], &v[15]); err != nil {
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

func (s *Server) Iterations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rows, _ := s.DB.Query("SELECT id,run_id,iteration_number,objective_snapshot,plan,reasoning_summary,action,action_result,reflection,status,started_at FROM iterations ORDER BY id DESC LIMIT 100")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, rid, iter sql.NullInt64
			var snap, plan, rs, act, res, ref, st, at sql.NullString
			rows.Scan(&id, &rid, &iter, &snap, &plan, &rs, &act, &res, &ref, &st, &at)
			out = append(out, map[string]any{"id": id.Int64, "run_id": rid.Int64, "iteration": iter.Int64, "prompt": snap.String, "ai_raw": plan.String, "tool_params": rs.String, "action": act.String, "action_result": res.String, "reflection": ref.String, "status": st.String, "started_at": at.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}
func (s *Server) SearchHistory(w http.ResponseWriter, r *http.Request) {
	rows, _ := s.DB.Query("SELECT id,objective_id,query,filters_json,result_count,created_at FROM search_history ORDER BY id DESC LIMIT 100")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, oid, rc sql.NullInt64
			var q, fj, at sql.NullString
			rows.Scan(&id, &oid, &q, &fj, &rc, &at)
			out = append(out, map[string]any{"id": id.Int64, "objective_id": oid.Int64, "query": q.String, "filters": fj.String, "result_count": rc.Int64, "created_at": at.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}
func (s *Server) LeadStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var cur sql.NullString
	s.DB.QueryRow("SELECT status FROM leads WHERE id=?", id).Scan(&cur)
	var body struct {
		NewStatus string `json:"new_status"`
		Reason    string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	to := body.NewStatus
	if to == "" {
		http.Error(w, `{"error":"new_status required"}`, 400)
		return
	}
	from := cur.String
	if from == "" {
		from = "NEW"
	}
	// simple validation
	allowed := map[string]bool{"NEW": true, "DISCOVERED": true, "ENRICHING": true, "QUALIFIED": true, "VERIFIED": true, "READY": true, "CONTACTED": true, "ENGAGED": true, "CONVERTED": true, "DISQUALIFIED": true, "INVALID": true, "DUPLICATE": true, "ARCHIVED": true, "DO_NOT_CONTACT": true}
	if !allowed[to] {
		http.Error(w, `{"error":"invalid status"}`, 400)
		return
	}
	// enforce the lifecycle state machine (ARCHIVED and DO_NOT_CONTACT are reachable from anywhere)
	if to != "DO_NOT_CONTACT" && !leads.CanTransition(from, to) {
		http.Error(w, `{"error":"invalid transition `+from+` -> `+to+`"}`, 400)
		return
	}
	// record history
	s.DB.Exec("UPDATE leads SET status=? WHERE id=?", to, id)
	s.DB.Exec("INSERT INTO lead_status_history(lead_id,old_status,new_status,reason) VALUES(?,?,?,?)", id, from, to, body.Reason)
	s.DB.Exec("INSERT INTO audit_log(event_type,lead_id,message) VALUES('lead.status',?,?)", id, "status "+from+"->"+to)
	json.NewEncoder(w).Encode(map[string]any{"id": id, "old": from, "new": to})
}
func (s *Server) LeadScoreExplain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var icp, dq, overall sql.NullInt64
	var title, ind, cc, dom, em sql.NullString
	s.DB.QueryRow("SELECT position_title,industry_name,country_code,company_domain,email,icp_score,data_quality_score,overall_score FROM leads WHERE id=?", id).Scan(&title, &ind, &cc, &dom, &em, &icp, &dq, &overall)
	json.NewEncoder(w).Encode(map[string]any{"lead_id": id, "icp": icp.Int64, "data_quality": dq.Int64, "overall": overall.Int64, "factors": []any{
		map[string]any{"name": "title_match", "value": title.String},
		map[string]any{"name": "industry", "value": ind.String},
		map[string]any{"name": "location", "value": cc.String},
	}})
}
func (s *Server) AdvancedSearch(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Filters []leads.Filter `json:"filters"`
		Limit   int            `json:"limit"`
		Offset  int            `json:"offset"`
	}
	json.NewDecoder(r.Body).Decode(&q)
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > 250 {
		q.Limit = 250
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	where, args, err := leads.BuildWhere(q.Filters)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	rows, err := s.DB.Query("SELECT id,first_name,last_name,email,company_name,position_title,overall_score FROM leads WHERE "+where+" LIMIT ? OFFSET ?", append(args, q.Limit, q.Offset)...)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, sc int64
		var fn, ln, em, co, ti sql.NullString
		rows.Scan(&id, &fn, &ln, &em, &co, &ti, &sc)
		out = append(out, map[string]any{"id": id, "first_name": fn.String, "last_name": ln.String, "email": em.String, "company_name": co.String, "position_title": ti.String, "overall_score": sc})
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(map[string]any{"data": out})
}
func (s *Server) Tags(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var b struct {
			Name string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		s.DB.Exec("INSERT OR IGNORE INTO tags(name,slug) VALUES(?,?)", b.Name, b.Name)
		json.NewEncoder(w).Encode(map[string]any{"name": b.Name})
		return
	}
	rows, _ := s.DB.Query("SELECT id,name FROM tags")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var n string
			rows.Scan(&id, &n)
			out = append(out, map[string]any{"id": id, "name": n})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}
func (s *Server) AnalyticsFunnel(w http.ResponseWriter, r *http.Request) {
	var total, qualified, verified, ready int
	s.DB.QueryRow("SELECT count(*) FROM leads").Scan(&total)
	s.DB.QueryRow("SELECT count(*) FROM leads WHERE overall_score>=50").Scan(&qualified)
	s.DB.QueryRow("SELECT count(*) FROM leads WHERE email_status='valid'").Scan(&verified)
	s.DB.QueryRow("SELECT count(*) FROM leads WHERE status='READY'").Scan(&ready)
	json.NewEncoder(w).Encode(map[string]any{"funnel": []any{
		map[string]any{"stage": "Discovered", "count": total},
		map[string]any{"stage": "Qualified", "count": qualified},
		map[string]any{"stage": "Verified", "count": verified},
		map[string]any{"stage": "Ready", "count": ready},
	}})
}
func (s *Server) Segments(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// accept filters as raw
		var raw map[string]any
		json.NewDecoder(r.Body).Decode(&raw)
		name, _ := raw["name"].(string)
		desc, _ := raw["description"].(string)
		fj, _ := json.Marshal(raw["filters"])
		if len(fj) == 0 || string(fj) == "null" {
			fj, _ = json.Marshal(raw["filters_json"])
		}
		s.DB.Exec("INSERT INTO segments(name,description,filters_json) VALUES(?,?,?)", name, desc, string(fj))
		json.NewEncoder(w).Encode(map[string]any{"name": name})
		return
	}
	rows, _ := s.DB.Query("SELECT id,name,description,filters_json,lead_count FROM segments ORDER BY id DESC")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var n, d, fj sql.NullString
			var lc sql.NullInt64
			rows.Scan(&id, &n, &d, &fj, &lc)
			out = append(out, map[string]any{"id": id, "name": n.String, "description": d.String, "filters": fj.String, "lead_count": lc.Int64})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}
func (s *Server) SegmentLeads(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var fj string
	if err := s.DB.QueryRow("SELECT filters_json FROM segments WHERE id=?", id).Scan(&fj); err != nil {
		http.Error(w, `{"error":"segment not found"}`, 404)
		return
	}
	// filters_json is stored either as a bare array of filters or as
	// {"filters": [...]}; accept both.
	var filters []leads.Filter
	if fj != "" {
		var arr []leads.Filter
		if json.Unmarshal([]byte(fj), &arr) != nil {
			var obj struct {
				Filters []leads.Filter `json:"filters"`
			}
			if json.Unmarshal([]byte(fj), &obj) == nil {
				filters = obj.Filters
			}
		} else {
			filters = arr
		}
	}
	where, args, err := leads.BuildWhere(filters)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	rows, err := s.DB.Query("SELECT id,first_name,last_name,email FROM leads WHERE "+where+" LIMIT 50", args...)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, 400)
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var i int64
		var fn, ln, em sql.NullString
		rows.Scan(&i, &fn, &ln, &em)
		out = append(out, map[string]any{"id": i, "first_name": fn.String, "last_name": ln.String, "email": em.String})
	}
	if out == nil {
		out = []map[string]any{}
	}
	s.DB.Exec("UPDATE segments SET lead_count=(SELECT count(*) FROM leads WHERE "+where+"), last_calculated=CURRENT_TIMESTAMP WHERE id=?", append(args, id)...)
	json.NewEncoder(w).Encode(map[string]any{"segment_id": id, "filters": fj, "data": out})
}
func (s *Server) SavedSearches(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var raw map[string]any
		json.NewDecoder(r.Body).Decode(&raw)
		fj, _ := json.Marshal(raw["filters"])
		sj, _ := json.Marshal(raw["sort"])
		name, _ := raw["name"].(string)
		s.DB.Exec("INSERT INTO saved_searches(name,filters_json,sort_json) VALUES(?,?,?)", name, string(fj), string(sj))
		json.NewEncoder(w).Encode(map[string]any{"name": name})
		return
	}
	rows, _ := s.DB.Query("SELECT id,name,filters_json FROM saved_searches ORDER BY id DESC")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var n, fj sql.NullString
			rows.Scan(&id, &n, &fj)
			out = append(out, map[string]any{"id": id, "name": n.String, "filters": fj.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}
func (s *Server) BulkOperation(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Type    string  `json:"type"`
		LeadIDs []int64 `json:"lead_ids"`
	}
	json.NewDecoder(r.Body).Decode(&b)
	if b.Type == "" {
		b.Type = "verify"
	}
	ids, _ := json.Marshal(b.LeadIDs)
	res, _ := s.DB.Exec("INSERT INTO bulk_operations(type,lead_ids_json,total,status) VALUES(?,?,?,?)", b.Type, string(ids), len(b.LeadIDs), "queued")
	id, _ := res.LastInsertId()
	go func() {
		s.DB.Exec("UPDATE bulk_operations SET status='processing' WHERE id=?", id)
		for i, lid := range b.LeadIDs {
			if b.Type == "verify" {
				if s.VerifyEmail != nil {
					var email string
					if err := s.DB.QueryRow("SELECT email FROM leads WHERE id=?", lid).Scan(&email); err == nil && email != "" {
						status, err := s.VerifyEmail(context.Background(), email)
						if err != nil {
							status = "unknown"
						}
						s.DB.Exec("UPDATE leads SET email_status=?, email_verified_at=CURRENT_TIMESTAMP WHERE id=?", status, lid)
						continue
					}
				}
				s.DB.Exec("UPDATE leads SET email_status='unknown' WHERE id=?", lid)
			}
			if b.Type == "archive" {
				s.DB.Exec("UPDATE leads SET status='ARCHIVED' WHERE id=?", lid)
			}
			s.DB.Exec("UPDATE bulk_operations SET progress=?, success_count=? WHERE id=?", i+1, i+1, id)
		}
		s.DB.Exec("UPDATE bulk_operations SET status='completed', completed_at=CURRENT_TIMESTAMP WHERE id=?", id)
	}()
	json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "queued", "total": len(b.LeadIDs)})
}
func (s *Server) BulkStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var st string
	var prog, total, succ int
	s.DB.QueryRow("SELECT status,progress,total,success_count FROM bulk_operations WHERE id=?", id).Scan(&st, &prog, &total, &succ)
	json.NewEncoder(w).Encode(map[string]any{"id": id, "status": st, "progress": prog, "total": total, "success": succ})
}
func (s *Server) ResearchQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var b struct {
			LeadID int64 `json:"lead_id"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		s.DB.Exec("INSERT INTO research_queue(lead_id,status) VALUES(?,?)", b.LeadID, "QUEUED")
		json.NewEncoder(w).Encode(map[string]any{"lead_id": b.LeadID, "status": "QUEUED"})
		return
	}
	rows, _ := s.DB.Query("SELECT id,lead_id,status FROM research_queue ORDER BY id DESC LIMIT 100")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, lid int64
			var st string
			rows.Scan(&id, &lid, &st)
			out = append(out, map[string]any{"id": id, "lead_id": lid, "status": st})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}
func (s *Server) Imports(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, `{"error":"multipart form expected"}`, 400)
			return
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"file field required"}`, 400)
			return
		}
		defer file.Close()
		filename := "upload.csv"
		if hdr != nil && hdr.Filename != "" {
			filename = hdr.Filename
		}
		res, err := s.DB.Exec("INSERT INTO imports(filename,format,status) VALUES(?,?,?)", filename, "csv", "processing")
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, 500)
			return
		}
		importID, _ := res.LastInsertId()

		parsed, err := leads.ParseLeadsCSV(file)
		if err != nil {
			s.DB.Exec("UPDATE imports SET status='failed' WHERE id=?", importID)
			http.Error(w, `{"error":"could not parse CSV: `+err.Error()+`"}`, 400)
			return
		}
		imported, duplicates := 0, 0
		seen := map[string]bool{}
		for _, l := range parsed {
			key := strings.ToLower(l.Email)
			if seen[key] {
				duplicates++
				continue
			}
			seen[key] = true
			res, err := s.DB.Exec(`INSERT INTO leads(first_name,last_name,email,company_name,company_domain,position_title,industry_name,country_code,city,linkedin_url,source,email_status)
				SELECT ?,?,?,?,?,?,?,?,?,?,?,? WHERE NOT EXISTS (SELECT 1 FROM leads WHERE email=?)`,
				l.FirstName, l.LastName, l.Email, l.Company, l.Domain, l.Title, l.Industry, l.Country, l.City, l.Linkedin, "import", "unchecked", l.Email)
			if err != nil {
				continue
			}
			if n, _ := res.RowsAffected(); n > 0 {
				imported++
			} else {
				duplicates++
			}
		}
		s.DB.Exec("UPDATE imports SET status='completed', total_rows=?, imported_rows=?, duplicate_rows=?, valid_rows=? WHERE id=?",
			len(parsed), imported, duplicates, imported, importID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": importID, "filename": filename, "status": "completed", "total_rows": len(parsed), "imported_rows": imported, "duplicate_rows": duplicates})
		return
	}
	rows, _ := s.DB.Query("SELECT id,filename,status,total_rows,imported_rows,duplicate_rows FROM imports ORDER BY id DESC")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var fn, st sql.NullString
			var total, imp, dup sql.NullInt64
			rows.Scan(&id, &fn, &st, &total, &imp, &dup)
			out = append(out, map[string]any{"id": id, "filename": fn.String, "status": st.String, "total_rows": total.Int64, "imported_rows": imp.Int64, "duplicate_rows": dup.Int64})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}
func (s *Server) ExportProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var raw map[string]any
		json.NewDecoder(r.Body).Decode(&raw)
		fj, _ := json.Marshal(raw["fields"])
		name, _ := raw["name"].(string)
		s.DB.Exec("INSERT INTO export_profiles(name,fields_json) VALUES(?,?)", name, string(fj))
		json.NewEncoder(w).Encode(map[string]any{"name": name})
		return
	}
	rows, _ := s.DB.Query("SELECT id,name,fields_json FROM export_profiles")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var n, fj sql.NullString
			rows.Scan(&id, &n, &fj)
			out = append(out, map[string]any{"id": id, "name": n.String, "fields": fj.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(out)
}
func (s *Server) GapAnalysis(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Target int `json:"target"`
	}
	json.NewDecoder(r.Body).Decode(&b)
	if b.Target == 0 {
		b.Target = 100
	}
	var total int
	s.DB.QueryRow("SELECT count(*) FROM leads").Scan(&total)
	gap := b.Target - total
	if gap < 0 {
		gap = 0
	}
	json.NewEncoder(w).Encode(map[string]any{"target": b.Target, "current": total, "gap": gap, "recommendations": []string{"Increase search diversification", "Verify emails to qualify more"}})
}
func (s *Server) StaleLeads(w http.ResponseWriter, r *http.Request) {
	rows, _ := s.DB.Query("SELECT id,email FROM leads WHERE freshness_status='STALE' LIMIT 50")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var em sql.NullString
			rows.Scan(&id, &em)
			out = append(out, map[string]any{"id": id, "email": em.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(map[string]any{"stale": out})
}
func (s *Server) SimilarLeads(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var ind sql.NullString
	s.DB.QueryRow("SELECT industry_name FROM leads WHERE id=?", id).Scan(&ind)
	rows, _ := s.DB.Query("SELECT id,first_name,last_name,position_title FROM leads WHERE industry_name=? AND id!=? LIMIT 10", ind.String, id)
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var i int64
			var fn, ln, ti sql.NullString
			rows.Scan(&i, &fn, &ln, &ti)
			out = append(out, map[string]any{"id": i, "first_name": fn.String, "last_name": ln.String, "title": ti.String, "similarity": 75})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(map[string]any{"lead_id": id, "similar": out})
}
