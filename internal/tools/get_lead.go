package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// GetLeadTool fetches one stored lead by email or numeric id so the agent can
// inspect what it already knows before spending an external API call.
type GetLeadTool struct{ db *sql.DB }

func NewGetLead(db *sql.DB) *GetLeadTool { return &GetLeadTool{db: db} }
func (t *GetLeadTool) Name() string      { return "get_lead" }
func (t *GetLeadTool) Description() string {
	return "Fetch one stored lead by {\"email\":\"a@b.com\"} or {\"id\":123}. Use before re-searching or re-verifying."
}
func (t *GetLeadTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var p struct {
		Email string `json:"email"`
		ID    int64  `json:"id"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return nil, err
	}
	if p.Email == "" && p.ID == 0 {
		return nil, fmt.Errorf("get_lead needs email or id")
	}
	q := `SELECT id,COALESCE(external_key,''),COALESCE(first_name,''),COALESCE(last_name,''),COALESCE(email,''),COALESCE(email_status,''),COALESCE(company_name,''),COALESCE(company_domain,''),COALESCE(position_title,''),COALESCE(industry_name,''),COALESCE(country_code,''),COALESCE(city,''),COALESCE(linkedin_url,''),COALESCE(summary,''),lead_score,COALESCE(status,''),COALESCE(source,'') FROM leads WHERE `
	args := []any{}
	if p.ID != 0 {
		q += "id=?"
		args = append(args, p.ID)
	} else {
		q += "email=?"
		args = append(args, p.Email)
	}
	q += " ORDER BY id DESC LIMIT 1"
	var m map[string]any
	row := t.db.QueryRowContext(ctx, q, args...)
	var id int64
	var ext, fn, ln, email, estatus, comp, dom, title, ind, cc, city, li, summary, status, source string
	var score int
	if err := row.Scan(&id, &ext, &fn, &ln, &email, &estatus, &comp, &dom, &title, &ind, &cc, &city, &li, &summary, &score, &status, &source); err != nil {
		if err == sql.ErrNoRows {
			return map[string]any{"found": false}, nil
		}
		return nil, err
	}
	m = map[string]any{"found": true, "id": id, "external_key": ext, "first_name": fn, "last_name": ln, "email": email, "email_status": estatus, "company_name": comp, "company_domain": dom, "position_title": title, "industry_name": ind, "country_code": cc, "city": city, "linkedin_url": li, "summary": summary, "lead_score": score, "status": status, "source": source}
	return m, nil
}
