package tools

import (
	"context"
	"database/sql"
	"encoding/json"
)

type StatsTool struct { db *sql.DB }
func NewStats(db *sql.DB) *StatsTool { return &StatsTool{db: db} }
func (t *StatsTool) Name() string { return "get_statistics" }
func (t *StatsTool) Description() string { return "Get lead statistics" }
func (t *StatsTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var row struct{ Total int `json:"total"`; Qualified int `json:"qualified"`}
	_ = t.db.QueryRowContext(ctx,"SELECT count(*) FROM leads").Scan(&row.Total)
	_ = t.db.QueryRowContext(ctx,"SELECT count(*) FROM leads WHERE lead_score>=70").Scan(&row.Qualified)
	return row,nil
}
