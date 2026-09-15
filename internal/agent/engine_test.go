package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"research-leads/internal/db"
	"research-leads/internal/emailvalidator"
	"research-leads/internal/events"
	"research-leads/internal/tools"
)

func newTestEngine(t *testing.T) (*Engine, func()) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	reg := tools.NewRegistry()
	reg.Register(tools.NewVerify(emailvalidator.New("", "", 5)))
	bus := events.New()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := New(d, nil, reg, bus, log, 10)
	return e, func() { d.Close() }
}

func TestPlanFallbackDerivesParamsFromObjective(t *testing.T) {
	e, done := newTestEngine(t)
	defer done()
	target := 5
	e.db.Exec(`INSERT INTO objectives(id,name,description,status,target_leads,minimum_score) VALUES(1,'o','Find CTOs at software companies in Romania','idle',?,70)`, target)
	a, prompt, _ := e.plan(context.Background(), 1)
	if a.Action != "search_leads" {
		t.Fatalf("expected fallback search_leads, got %s", a.Action)
	}
	var p map[string]any
	if err := json.Unmarshal(a.Parameters, &p); err != nil {
		t.Fatal(err)
	}
	if p["q"] != "cto" || p["country_code"] != "RO" {
		t.Fatalf("fallback must derive from the objective, got %v", p)
	}
	if !strings.Contains(prompt, "Find CTOs at software companies in Romania") {
		t.Fatalf("prompt must carry the objective text")
	}
}

func TestPlanCompletesWhenTargetMet(t *testing.T) {
	e, done := newTestEngine(t)
	defer done()
	e.db.Exec(`INSERT INTO objectives(id,name,description,status,target_leads,minimum_score) VALUES(1,'o','Find CTOs','idle',1,70)`)
	e.db.Exec(`INSERT INTO leads(external_key,email,lead_score,objective_id) VALUES('k1','a@x.com',80,1)`)
	a, _, _ := e.plan(context.Background(), 1)
	if a.Action != "complete_objective" {
		t.Fatalf("met target must complete, got %s", a.Action)
	}
}

func TestHandleResultDedupesIdlessLeadsByEmail(t *testing.T) {
	e, done := newTestEngine(t)
	defer done()
	e.db.Exec(`INSERT INTO objectives(id,name,description,status) VALUES(1,'o','x','running')`)
	result := []map[string]any{
		{"email": "dup@acme.com", "first_name": "Dup", "position_title": "CTO"},
	}
	params := json.RawMessage(`{"q":"CTO","country_code":"GB"}`)
	e.handleResult(context.Background(), 1, "search_leads", params, result)
	e.handleResult(context.Background(), 1, "search_leads", params, result)
	var n int
	e.db.QueryRow(`SELECT count(*) FROM leads WHERE email='dup@acme.com'`).Scan(&n)
	if n != 1 {
		t.Fatalf("id-less leads must dedupe by email, got %d rows", n)
	}
	var key string
	e.db.QueryRow(`SELECT external_key FROM leads WHERE email='dup@acme.com'`).Scan(&key)
	if key != "email:dup@acme.com" {
		t.Fatalf("expected email fallback key, got %q", key)
	}
	var q string
	e.db.QueryRow(`SELECT query FROM search_history ORDER BY id DESC LIMIT 1`).Scan(&q)
	if q == "search" || q == "" {
		t.Fatalf("search history must record the real query, got %q", q)
	}
}

func TestHandleResultSkipsReverifyOfKnownVerdict(t *testing.T) {
	e, done := newTestEngine(t)
	defer done()
	e.db.Exec(`INSERT INTO objectives(id,name,description,status) VALUES(1,'o','x','running')`)
	e.db.Exec(`INSERT INTO leads(external_key,email,email_status) VALUES('k0','dup@acme.com','valid')`)
	result := []map[string]any{{"email": "dup@acme.com", "position_title": "CTO"}}
	e.handleResult(context.Background(), 1, "search_leads", json.RawMessage(`{"q":"CTO"}`), result)
	var st string
	e.db.QueryRow(`SELECT email_status FROM leads WHERE email='dup@acme.com'`).Scan(&st)
	if st != "valid" {
		t.Fatalf("existing verdict must not be re-verified/overwritten, got %q", st)
	}
}
