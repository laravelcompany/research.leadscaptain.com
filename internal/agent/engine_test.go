package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"research-leads/internal/db"
	"research-leads/internal/events"
	"research-leads/internal/scoring"
	"research-leads/internal/tools"
)

// stubVerifyTool returns a fixed email verdict so ingestion tests stay offline.
type stubVerifyTool struct{ status string }

func (s stubVerifyTool) Name() string        { return "verify_email" }
func (s stubVerifyTool) Description() string { return "stub email verification" }
func (s stubVerifyTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	return map[string]any{"email": "x", "status": s.status}, nil
}

// stubWebsiteTool reports every domain as a live site.
type stubWebsiteTool struct{ reachable, parked bool }

func (s stubWebsiteTool) Name() string        { return "check_website" }
func (s stubWebsiteTool) Description() string { return "stub website check" }
func (s stubWebsiteTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	code := 200.0
	if !s.reachable {
		code = 0
	}
	return map[string]any{"reachable": s.reachable, "status_code": code, "parked": s.parked}, nil
}

func newTestEngine(t *testing.T) (*Engine, func()) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	reg := tools.NewRegistry()
	reg.Register(stubVerifyTool{status: "valid"})
	reg.Register(stubWebsiteTool{reachable: true})
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

func TestHandleResultChecksWebsiteThenEmailThenScores(t *testing.T) {
	e, done := newTestEngine(t)
	defer done()
	e.db.Exec(`INSERT INTO objectives(id,name,description,status) VALUES(1,'o','x','running')`)
	result := []map[string]any{{
		"email": "cto@acme.com", "first_name": "Amy", "position_title": "CTO",
		"industry_name": "Software", "country_code": "GB", "company_domain": "acme.com",
		"summary": "Leads the engineering org and owns the platform roadmap.",
	}}
	e.handleResult(context.Background(), 1, "search_leads", json.RawMessage(`{"q":"CTO"}`), result)
	var es, ws string
	var score int
	e.db.QueryRow(`SELECT email_status, website_status, lead_score FROM leads WHERE email='cto@acme.com'`).Scan(&es, &ws, &score)
	if es != "valid" {
		t.Fatalf("email must be verified at ingestion, got %q", es)
	}
	if ws != "live" {
		t.Fatalf("website must be checked at ingestion, got %q", ws)
	}
	want := scoring.ScoreWithSummary("CTO", "Software", "GB", "valid", "acme.com", "live", "Leads the engineering org and owns the platform roadmap.", []string{"CTO"}, []string{"Software"}, []string{"GB"})
	want.Score = adjustForSummary(want.Score, "Leads the engineering org and owns the platform roadmap.")
	if score != want.Score {
		t.Fatalf("score must come from the scorer with real statuses: got %d, want %d", score, want.Score)
	}
}

func TestHandleResultInvalidEmailRescoredNotClamped(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	reg := tools.NewRegistry()
	reg.Register(stubVerifyTool{status: "invalid"})
	reg.Register(stubWebsiteTool{reachable: false})
	bus := events.New()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	e := New(d, nil, reg, bus, log, 10)

	e.db.Exec(`INSERT INTO objectives(id,name,description,status) VALUES(1,'o','x','running')`)
	// A pre-existing unchecked row for the same address must be rescored too.
	e.db.Exec(`INSERT INTO leads(external_key,email,position_title,industry_name,country_code,company_domain,email_status,website_status,lead_score,summary) VALUES('k9','bob@acme.com','CTO','Software','GB','acme.com','unchecked','unchecked',42,'Experienced technology leader with a long track record.')`)
	result := []map[string]any{{
		"email": "bob@acme.com", "position_title": "CTO", "industry_name": "Software",
		"country_code": "GB", "company_domain": "acme.com",
		"summary": "Experienced technology leader with a long track record.",
	}}
	e.handleResult(context.Background(), 1, "search_leads", json.RawMessage(`{"q":"CTO"}`), result)

	rows, _ := e.db.Query(`SELECT email_status, website_status, lead_score FROM leads WHERE email='bob@acme.com'`)
	defer rows.Close()
	summary := "Experienced technology leader with a long track record."
	want := scoring.ScoreWithSummary("CTO", "Software", "GB", "invalid", "acme.com", "unreachable", summary, []string{"CTO"}, []string{"Software"}, []string{"GB"})
	want.Score = adjustForSummary(want.Score, summary)
	n := 0
	for rows.Next() {
		var es, ws string
		var score int
		rows.Scan(&es, &ws, &score)
		n++
		if es != "invalid" || ws != "unreachable" {
			t.Fatalf("verdicts must propagate to every row: got %q/%q", es, ws)
		}
		if score != want.Score {
			t.Fatalf("score must be recomputed, not clamped to 10: got %d, want %d", score, want.Score)
		}
		if score == 10 {
			t.Fatal("hard-coded score 10 must be gone")
		}
	}
	if n != 2 {
		t.Fatalf("expected 2 rows (pre-existing + inserted), got %d", n)
	}
	var hist int
	e.db.QueryRow(`SELECT count(*) FROM lead_score_history h JOIN leads l ON l.id=h.lead_id WHERE l.email='bob@acme.com'`).Scan(&hist)
	if hist == 0 {
		t.Fatal("score drift from verification must land in lead_score_history")
	}
}
