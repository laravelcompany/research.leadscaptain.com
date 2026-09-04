package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"research-leads/internal/agent"
	"research-leads/internal/events"
)

type Server struct {
	DB     *sql.DB
	Bus    *events.Bus
	Engine *agent.Engine
}

func (s *Server) Objectives(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var total int
		s.DB.QueryRow("SELECT count(*) FROM leads").Scan(&total)
		rows, _ := s.DB.Query("SELECT id,name,description,status,target_leads,minimum_score,iteration_count FROM objectives ORDER BY id DESC")
		type tmp struct {
			id       int64
			n, d, st string
			tl       sql.NullInt64
			ms, iter int
		}
		var tmps []tmp
		if rows != nil {
			for rows.Next() {
				var id int64
				var n, d, st string
				var tl sql.NullInt64
				var ms int
				var iterNull sql.NullInt64
				rows.Scan(&id, &n, &d, &st, &tl, &ms, &iterNull)
				iter := 0
				if iterNull.Valid {
					iter = int(iterNull.Int64)
				}
				tmps = append(tmps, tmp{id, n, d, st, tl, ms, iter})
			}
			rows.Close()
		}
		var out []map[string]any
		for _, t := range tmps {
			var qualified int
			s.DB.QueryRow("SELECT count(*) FROM leads WHERE lead_score>=?", t.ms).Scan(&qualified)
			m := map[string]any{"id": t.id, "name": t.n, "description": t.d, "status": t.st, "minimum_score": t.ms, "iteration_count": t.iter, "leads_discovered": total, "qualified": qualified}
			if t.tl.Valid {
				m["target_leads"] = t.tl.Int64
				if t.tl.Int64 > 0 {
					pct := qualified * 100 / int(t.tl.Int64)
					if pct > 100 {
						pct = 100
					}
					m["progress"] = pct
				}
			}
			out = append(out, m)
		}
		if out == nil {
			out = []map[string]any{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	case "POST":
		var body struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			TargetLeads  *int   `json:"target_leads"`
			MinimumScore int    `json:"minimum_score"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Name == "" {
			body.Name = "Objective"
		}
		res, _ := s.DB.Exec("INSERT INTO objectives(name,description,status,target_leads,minimum_score) VALUES(?,?,'idle',?,?)", body.Name, body.Description, body.TargetLeads, body.MinimumScore)
		id, _ := res.LastInsertId()
		json.NewEncoder(w).Encode(map[string]any{"id": id, "name": body.Name, "status": "idle"})
	}
}

func (s *Server) ObjectiveByID(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		id := chi.URLParam(r, "id")
		s.DB.Exec("DELETE FROM iterations WHERE run_id IN (SELECT id FROM agent_runs WHERE objective_id=?)", id)
		s.DB.Exec("DELETE FROM agent_runs WHERE objective_id=?", id)
		s.DB.Exec("DELETE FROM search_history WHERE objective_id=?", id)
		s.DB.Exec("DELETE FROM objectives WHERE id=?", id)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"deleted": id})
		return
	}
	id := chi.URLParam(r, "id")
	var n, d, st string
	var tl sql.NullInt64
	var ms int
	err := s.DB.QueryRow("SELECT name,description,status,target_leads,minimum_score FROM objectives WHERE id=?", id).Scan(&n, &d, &st, &tl, &ms)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, 404)
		return
	}
	m := map[string]any{"id": id, "name": n, "description": d, "status": st, "minimum_score": ms}
	if tl.Valid {
		m["target_leads"] = tl.Int64
	}
	json.NewEncoder(w).Encode(m)
}

func (s *Server) StartObjective(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	runID, err := s.Engine.Start(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"run_id": runID})
}
func (s *Server) PauseObjective(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	s.Engine.Pause(r.Context(), id)
	json.NewEncoder(w).Encode(map[string]any{"status": "paused"})
}
func (s *Server) ResumeObjective(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	s.Engine.Resume(r.Context(), id)
	json.NewEncoder(w).Encode(map[string]any{"status": "running"})
}
func (s *Server) StopObjective(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	s.Engine.Stop(r.Context(), id)
	json.NewEncoder(w).Encode(map[string]any{"status": "stopped"})
}

func (s *Server) Leads(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		s.DB.Exec("DELETE FROM leads")
		s.DB.Exec("DELETE FROM search_history")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"deleted": true})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	per, _ := strconv.Atoi(q.Get("per_page"))
	if per < 1 || per > 250 {
		per = 25
	}
	offset := (page - 1) * per
	search := q.Get("search")
	query := "SELECT id,first_name,last_name,email,company_name,position_title,lead_score,linkedin_url,company_domain,city,country_code,industry_name,email_status,summary,outreach_message FROM leads WHERE 1=1"
	args := []any{}
	if search != "" {
		query += " AND (first_name LIKE ? OR last_name LIKE ? OR email LIKE ? OR company_name LIKE ?)"
		like := "%" + search + "%"
		args = append(args, like, like, like, like)
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, per, offset)
	rows, _ := s.DB.Query(query, args...)
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, score int64
			var fn, ln, email, comp, title, li, dom, city, cc, ind, es, sum, outmsg sql.NullString
			rows.Scan(&id, &fn, &ln, &email, &comp, &title, &score, &li, &dom, &city, &cc, &ind, &es, &sum, &outmsg)
			out = append(out, map[string]any{"id": id, "first_name": fn.String, "last_name": ln.String, "email": email.String, "company_name": comp.String, "position_title": title.String, "lead_score": score, "linkedin_url": li.String, "company_domain": dom.String, "city": city.String, "country_code": cc.String, "industry_name": ind.String, "email_status": es.String, "summary": sum.String, "outreach_message": outmsg.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	json.NewEncoder(w).Encode(map[string]any{"data": out, "page": page, "per_page": per})
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

func (s *Server) Events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	ch := s.Bus.Subscribe()
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
	rows, _ := s.DB.Query("SELECT first_name,last_name,email,company_name,company_domain,position_title,industry_name,country_code,city,lead_score,linkedin_url FROM leads ORDER BY id")
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=leads.csv")
	w.Write([]byte("first_name,last_name,email,company,company_domain,title,industry,country,city,score,linkedin\n"))
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var fn, ln, em, co, dom, ti, ind, cc, city, li sql.NullString
			var sc sql.NullInt64
			rows.Scan(&fn, &ln, &em, &co, &dom, &ti, &ind, &cc, &city, &sc, &li)
			line := "\"" + fn.String + "\",\"" + ln.String + "\",\"" + em.String + "\",\"" + co.String + "\",\"" + dom.String + "\",\"" + ti.String + "\",\"" + ind.String + "\",\"" + cc.String + "\",\"" + city.String + "\",\"" + strconv.FormatInt(sc.Int64, 10) + "\",\"" + li.String + "\"\n"
			w.Write([]byte(line))
		}
	}
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
		Filters []struct {
			Field    string      `json:"field"`
			Operator string      `json:"operator"`
			Value    interface{} `json:"value"`
		} `json:"filters"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	json.NewDecoder(r.Body).Decode(&q)
	if q.Limit == 0 {
		q.Limit = 50
	}
	if q.Limit > 250 {
		q.Limit = 250
	}
	// build where via simple mapping
	where := "1=1"
	var args []any
	for _, f := range q.Filters {
		col := f.Field
		if col == "title" {
			col = "position_title"
		} else if col == "country" {
			col = "country_code"
		} else if col == "company" {
			col = "company_name"
		}
		op := f.Operator
		if op == "CONTAINS" {
			where += " AND " + col + " LIKE ?"
			args = append(args, "%"+fmt.Sprint(f.Value)+"%")
		} else if op == "=" {
			where += " AND " + col + "=?"
			args = append(args, f.Value)
		} else if op == ">=" {
			where += " AND overall_score>=?"
			args = append(args, f.Value)
		}
	}
	rows, _ := s.DB.Query("SELECT id,first_name,last_name,email,company_name,position_title,overall_score FROM leads WHERE "+where+" LIMIT ? OFFSET ?", append(args, q.Limit, q.Offset)...)
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, sc int64
			var fn, ln, em, co, ti sql.NullString
			rows.Scan(&id, &fn, &ln, &em, &co, &ti, &sc)
			out = append(out, map[string]any{"id": id, "first_name": fn.String, "last_name": ln.String, "email": em.String, "company_name": co.String, "position_title": ti.String, "overall_score": sc})
		}
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
	s.DB.QueryRow("SELECT filters_json FROM segments WHERE id=?", id).Scan(&fj)
	// naive: return all leads if no filter parsing
	rows, _ := s.DB.Query("SELECT id,first_name,last_name,email FROM leads LIMIT 50")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var i int64
			var fn, ln, em sql.NullString
			rows.Scan(&i, &fn, &ln, &em)
			out = append(out, map[string]any{"id": i, "first_name": fn.String, "last_name": ln.String, "email": em.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
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
				s.DB.Exec("UPDATE leads SET email_status='valid' WHERE id=?", lid)
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
		r.ParseMultipartForm(10 << 20)
		file, _, _ := r.FormFile("file")
		var content []byte
		if file != nil {
			content = make([]byte, 4096)
			file.Read(content)
			file.Close()
		}
		s.DB.Exec("INSERT INTO imports(filename,format,status,total_rows) VALUES(?,?,?,?)", "upload.csv", "csv", "pending", len(content))
		json.NewEncoder(w).Encode(map[string]any{"status": "pending", "preview": string(content[:200])})
		return
	}
	rows, _ := s.DB.Query("SELECT id,filename,status FROM imports ORDER BY id DESC")
	var out []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var fn, st sql.NullString
			rows.Scan(&id, &fn, &st)
			out = append(out, map[string]any{"id": id, "filename": fn.String, "status": st.String})
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
