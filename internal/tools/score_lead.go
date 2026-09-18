package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"research-leads/internal/scoring"
)

// ScoreLeadTool recomputes a stored lead's score with the same scorer the
// ingestion path uses, persists it, and records the change in
// lead_score_history so score drift is auditable.
type ScoreLeadTool struct{ db *sql.DB }

func NewScoreLead(db *sql.DB) *ScoreLeadTool { return &ScoreLeadTool{db: db} }
func (t *ScoreLeadTool) Name() string        { return "score_lead" }
func (t *ScoreLeadTool) Description() string {
	return "Recompute and persist the score for one stored lead: {\"email\":\"a@b.com\"} or {\"id\":123}. Returns the new score and breakdown."
}
func (t *ScoreLeadTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var p struct {
		Email string `json:"email"`
		ID    int64  `json:"id"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return nil, err
	}
	if p.Email == "" && p.ID == 0 {
		return nil, fmt.Errorf("score_lead needs email or id")
	}
	q := `SELECT id,COALESCE(position_title,''),COALESCE(industry_name,''),COALESCE(country_code,''),COALESCE(city,''),COALESCE(email_status,''),COALESCE(company_domain,''),COALESCE(summary,''),COALESCE(website_status,''),lead_score FROM leads WHERE `
	args := []any{}
	if p.ID != 0 {
		q += "id=?"
		args = append(args, p.ID)
	} else {
		q += "email=?"
		args = append(args, p.Email)
	}
	q += " ORDER BY id DESC LIMIT 1"
	var id int64
	var title, ind, cc, city, estatus, dom, summary, wstatus string
	var oldScore int
	if err := t.db.QueryRowContext(ctx, q, args...).Scan(&id, &title, &ind, &cc, &city, &estatus, &dom, &summary, &wstatus, &oldScore); err != nil {
		if err == sql.ErrNoRows {
			return map[string]any{"found": false}, nil
		}
		return nil, err
	}
	loc := cc
	if loc == "" {
		loc = city
	}
	r := scoring.ScoreWithSummary(title, ind, loc, estatus, dom, wstatus, summary, []string{title}, []string{ind}, []string{cc, city})
	if _, err := t.db.ExecContext(ctx, "UPDATE leads SET lead_score=?, score_breakdown=?, updated_at=CURRENT_TIMESTAMP WHERE id=?", r.Score, r.JSON(), id); err != nil {
		return nil, err
	}
	if oldScore != r.Score {
		t.db.ExecContext(ctx, "INSERT INTO lead_score_history(lead_id,score_type,old_score,new_score,breakdown) VALUES(?,?,?,?,?)", id, "lead", oldScore, r.Score, r.JSON())
	}
	return map[string]any{"found": true, "id": id, "score": r.Score, "previous_score": oldScore, "breakdown": r.Breakdown}, nil
}
