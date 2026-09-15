package tools

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportLeadsTool writes the stored leads matching safe filters to a CSV file
// under the configured export directory and returns the path and row count.
type ExportLeadsTool struct {
	db  *sql.DB
	dir string
}

func NewExportLeads(db *sql.DB, dir string) *ExportLeadsTool {
	return &ExportLeadsTool{db: db, dir: dir}
}
func (t *ExportLeadsTool) Name() string { return "export_leads" }
func (t *ExportLeadsTool) Description() string {
	return "Export stored leads to CSV on disk. Optional filters: {\"objective_id\":1,\"min_score\":70,\"status\":\"NEW\"}. Returns {\"path\",\"count\"}."
}
func (t *ExportLeadsTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var p struct {
		ObjectiveID *int64 `json:"objective_id"`
		MinScore    *int   `json:"min_score"`
		Status      string `json:"status"`
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
	q := `SELECT COALESCE(email,''),COALESCE(first_name,''),COALESCE(last_name,''),COALESCE(company_name,''),COALESCE(company_domain,''),COALESCE(position_title,''),COALESCE(industry_name,''),COALESCE(country_code,''),COALESCE(city,''),COALESCE(linkedin_url,''),COALESCE(email_status,''),lead_score,COALESCE(status,''),COALESCE(source,'') FROM leads`
	if len(wh) > 0 {
		q += " WHERE " + strings.Join(wh, " AND ")
	}
	q += " ORDER BY lead_score DESC, id DESC"
	rows, err := t.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	dir := t.dir
	if dir == "" {
		dir = filepath.Join("data", "exports")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "leads-"+time.Now().UTC().Format("20060102-150405")+".csv")
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"email", "first_name", "last_name", "company_name", "company_domain", "position_title", "industry_name", "country_code", "city", "linkedin_url", "email_status", "lead_score", "status", "source"})
	count := 0
	for rows.Next() {
		var email, fn, ln, comp, dom, title, ind, cc, city, li, estatus, status, source string
		var score int
		if err := rows.Scan(&email, &fn, &ln, &comp, &dom, &title, &ind, &cc, &city, &li, &estatus, &score, &status, &source); err != nil {
			return nil, err
		}
		w.Write([]string{email, fn, ln, comp, dom, title, ind, cc, city, li, estatus, fmt.Sprint(score), status, source})
		count++
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "count": count}, rows.Err()
}
