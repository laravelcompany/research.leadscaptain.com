package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// ListLeadsTool lists stored leads with a small set of safe filters, so the
// agent can review progress without raw SQL.
type ListLeadsTool struct{ db *sql.DB }

func NewListLeads(db *sql.DB) *ListLeadsTool { return &ListLeadsTool{db: db} }
func (t *ListLeadsTool) Name() string        { return "list_leads" }
func (t *ListLeadsTool) Description() string {
	return "List stored leads. Optional filters: {\"objective_id\":1,\"min_score\":70,\"status\":\"NEW\",\"limit\":20}. Returns id/email/title/company/score/email_status rows."
}
func (t *ListLeadsTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var p struct {
		ObjectiveID *int64 `json:"objective_id"`
		MinScore    *int   `json:"min_score"`
		Status      string `json:"status"`
		EmailStatus string `json:"email_status"`
		Limit       int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &p); err != nil && len(input) > 0 {
		return nil, err
	}
	var wh []string
	var args []any
	if p.ObjectiveID != nil {
		wh = append(wh, "objective_id=?")
		args = append(args, *p.ObjectiveID)
	}
	if p.MinScore != nil {
		wh = append(wh, "lead_score>=?")
		args = append(args, *p.MinScore)
	}
	if p.Status != "" {
		if !safeWord(p.Status) {
			return nil, fmt.Errorf("invalid status filter")
		}
		wh = append(wh, "status=?")
		args = append(args, p.Status)
	}
	if p.EmailStatus != "" {
		if !safeWord(p.EmailStatus) {
			return nil, fmt.Errorf("invalid email_status filter")
		}
		wh = append(wh, "email_status=?")
		args = append(args, p.EmailStatus)
	}
	limit := p.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := `SELECT id,COALESCE(email,''),COALESCE(first_name,''),COALESCE(last_name,''),COALESCE(company_name,''),COALESCE(position_title,''),lead_score,COALESCE(email_status,''),COALESCE(status,'') FROM leads`
	if len(wh) > 0 {
		q += " WHERE " + strings.Join(wh, " AND ")
	}
	q += " ORDER BY lead_score DESC, id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := t.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var email, fn, ln, comp, title, estatus, status string
		var score int
		if err := rows.Scan(&id, &email, &fn, &ln, &comp, &title, &score, &estatus, &status); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "email": email, "first_name": fn, "last_name": ln, "company_name": comp, "position_title": title, "lead_score": score, "email_status": estatus, "status": status})
	}
	return map[string]any{"count": len(out), "leads": out}, rows.Err()
}

func safeWord(s string) bool {
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '-') {
			return false
		}
	}
	return len(s) > 0
}
