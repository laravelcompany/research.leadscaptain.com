package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"research-leads/internal/db"
)

func TestGetLeadByEmailAndNotFound(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Exec(`INSERT INTO leads(external_key,email,first_name,last_name,company_name,position_title,lead_score,email_status) VALUES('k1','ada@acme.com','Ada','Lovelace','Acme','CTO',80,'valid')`)
	tool := NewGetLead(d)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"email":"ada@acme.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["found"] != true || m["first_name"] != "Ada" {
		t.Fatalf("unexpected result: %v", m)
	}
	res, err = tool.Execute(context.Background(), json.RawMessage(`{"email":"nobody@nowhere.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.(map[string]any)["found"] != false {
		t.Fatalf("expected found=false, got %v", res)
	}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error when neither email nor id is given")
	}
}

func TestListLeadsFilters(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Exec(`INSERT INTO leads(external_key,email,first_name,company_name,lead_score,email_status,status) VALUES
		('k1','a@x.com','A','X',90,'valid','NEW'),
		('k2','b@x.com','B','X',40,'unchecked','NEW'),
		('k3','c@x.com','C','Y',75,'valid','CONTACTED')`)
	tool := NewListLeads(d)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"min_score":70}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.(map[string]any)["count"] != 2 {
		t.Fatalf("expected 2 leads above score 70, got %v", res)
	}
	res, err = tool.Execute(context.Background(), json.RawMessage(`{"status":"CONTACTED"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.(map[string]any)["count"] != 1 {
		t.Fatalf("expected 1 CONTACTED lead, got %v", res)
	}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"status":"NEW'; DROP TABLE leads;--"}`)); err == nil {
		t.Fatal("expected unsafe status filter to be rejected")
	}
}

func TestScoreLeadPersistsAndRecordsHistory(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Exec(`INSERT INTO leads(external_key,email,position_title,industry_name,country_code,email_status,company_domain,lead_score) VALUES
		('k1','ada@acme.com','CTO','Software','GB','valid','acme.com',10)`)
	tool := NewScoreLead(d)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"email":"ada@acme.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["found"] != true {
		t.Fatalf("expected lead found: %v", m)
	}
	if m["score"] != 85 {
		t.Fatalf("expected recomputed score 85 (no-summary penalty applied), got %v", m["score"])
	}
	var stored int
	d.QueryRow("SELECT lead_score FROM leads WHERE email='ada@acme.com'").Scan(&stored)
	if stored != 85 {
		t.Fatalf("score not persisted, got %d", stored)
	}
	var hist int
	d.QueryRow("SELECT count(*) FROM lead_score_history WHERE lead_id=1 AND new_score=85").Scan(&hist)
	if hist != 1 {
		t.Fatalf("expected 1 score-history row, got %d", hist)
	}
	res, _ = tool.Execute(context.Background(), json.RawMessage(`{"email":"ada@acme.com"}`))
	d.QueryRow("SELECT count(*) FROM lead_score_history WHERE lead_id=1").Scan(&hist)
	if hist != 1 {
		t.Fatalf("unchanged score must not add history rows, got %d", hist)
	}
}

func TestExportLeadsWritesCSV(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Exec(`INSERT INTO leads(external_key,email,first_name,company_name,lead_score) VALUES
		('k1','a@x.com','A','X',90),('k2','b@x.com','B','Y',40)`)
	dir := t.TempDir()
	tool := NewExportLeads(d, dir)
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"min_score":50}`))
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["count"] != 1 {
		t.Fatalf("expected 1 exported lead, got %v", m["count"])
	}
	path := m["path"].(string)
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("export must stay inside export dir, got %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	if !strings.Contains(content, "a@x.com") || strings.Contains(content, "b@x.com") {
		t.Fatalf("unexpected csv content:\n%s", content)
	}
	if !strings.HasPrefix(content, "email,first_name") {
		t.Fatalf("missing header row:\n%s", content)
	}
	if filepath.Ext(path) != ".csv" {
		t.Fatalf("expected .csv path, got %s", path)
	}
}
