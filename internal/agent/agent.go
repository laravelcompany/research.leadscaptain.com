package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"research-leads/internal/ai"
	"research-leads/internal/events"
	"research-leads/internal/scoring"
	"research-leads/internal/tools"
)

type Engine struct {
	db       *sql.DB
	ai       *ai.Client
	registry *tools.Registry
	bus      *events.Bus
	log      *slog.Logger
	maxIter  int
	mu       sync.Mutex
	// loopGen tracks the newest loop generation per objective; a superseded
	// loop (e.g. after pause+resume while an iteration was in flight) exits
	// instead of running in parallel with its replacement.
	loopGen map[int64]int
}
type pendingLead struct {
	Email, FirstName, LastName, Company, Title, Summary, City, Country string
}

func New(db *sql.DB, ai *ai.Client, reg *tools.Registry, bus *events.Bus, log *slog.Logger, maxIter int) *Engine {
	return &Engine{db: db, ai: ai, registry: reg, bus: bus, log: log, maxIter: maxIter, loopGen: map[int64]int{}}
}

type Action struct {
	Action     string          `json:"action"`
	Reason     string          `json:"reason"`
	Parameters json.RawMessage `json:"parameters"`
}

func (e *Engine) Start(ctx context.Context, objectiveID int64) (int64, error) {
	var objStatus string
	if err := e.db.QueryRowContext(ctx, "SELECT status FROM objectives WHERE id=?", objectiveID).Scan(&objStatus); err != nil {
		return 0, fmt.Errorf("objective %d: %w", objectiveID, err)
	}
	if objStatus == "running" {
		return 0, fmt.Errorf("objective %d is already running", objectiveID)
	}
	res, err := e.db.ExecContext(ctx, "INSERT INTO agent_runs(objective_id,status,started_at) VALUES(?,'running',CURRENT_TIMESTAMP)", objectiveID)
	if err != nil {
		return 0, err
	}
	runID, _ := res.LastInsertId()
	if _, err := e.db.ExecContext(ctx, "UPDATE objectives SET status='running', started_at=CURRENT_TIMESTAMP WHERE id=?", objectiveID); err != nil {
		return 0, err
	}
	e.bus.Publish(events.Event{Type: "agent.started", Payload: map[string]any{"objective_id": objectiveID, "run_id": runID}})
	go e.loop(runID, objectiveID, 0)
	return runID, nil
}

func (e *Engine) loop(runID, objectiveID int64, startIter int) {
	e.log.Info("agent loop started", "run_id", runID, "objective_id", objectiveID, "start_iter", startIter)
	ctx := context.Background()
	e.mu.Lock()
	e.loopGen[objectiveID]++
	gen := e.loopGen[objectiveID]
	e.mu.Unlock()
	// any 'running' iteration rows left by a superseded loop or a crash are
	// stale; mark them interrupted so the record stays honest.
	e.db.ExecContext(ctx, "UPDATE iterations SET status='interrupted', completed_at=CURRENT_TIMESTAMP WHERE run_id=? AND status='running'", runID)
	defer func() {
		if r := recover(); r != nil {
			e.log.Error("agent loop panic", "run_id", runID, "objective_id", objectiveID, "panic", r)
			e.db.ExecContext(context.Background(), "UPDATE agent_runs SET status='failed', finished_at=CURRENT_TIMESTAMP WHERE id=? AND status='running'", runID)
			e.db.ExecContext(context.Background(), "UPDATE objectives SET status='failed' WHERE id=? AND status='running'", objectiveID)
			e.bus.Publish(events.Event{Type: "agent.failed", Payload: map[string]any{"objective_id": objectiveID, "run_id": runID, "error": fmt.Sprint(r)}})
		}
	}()
	for i := startIter; i < e.maxIter; i++ {
		e.log.Debug("iteration start", "run_id", runID, "iter", i+1)
		e.mu.Lock()
		current := e.loopGen[objectiveID] == gen
		e.mu.Unlock()
		if !current {
			e.log.Info("agent loop superseded, exiting", "run_id", runID, "objective_id", objectiveID)
			return
		}
		var status string
		e.db.QueryRowContext(ctx, "SELECT status FROM objectives WHERE id=?", objectiveID).Scan(&status)
		if status == "paused" {
			e.db.ExecContext(ctx, "UPDATE agent_runs SET status='paused' WHERE id=?", runID)
			e.db.ExecContext(ctx, "UPDATE iterations SET status='interrupted', completed_at=CURRENT_TIMESTAMP WHERE run_id=? AND status='running'", runID)
			return
		}
		if status == "stopped" {
			e.db.ExecContext(ctx, "UPDATE agent_runs SET status='stopped', finished_at=CURRENT_TIMESTAMP WHERE id=?", runID)
			return
		}

		e.db.ExecContext(ctx, "INSERT INTO iterations(run_id,iteration_number,status,started_at) VALUES(?,?, 'running', CURRENT_TIMESTAMP)", runID, i+1)
		var iterID int64
		e.db.QueryRowContext(ctx, "SELECT last_insert_rowid()").Scan(&iterID)

		action, prompt, rawAI := e.plan(ctx, objectiveID)
		e.log.Info("planned", "run_id", runID, "iter", i+1, "action", action.Action, "reason", action.Reason, "prompt_len", len(prompt))
		e.log.Debug("ai raw", "raw", rawAI[:min(500, len(rawAI))])
		allowed := append(e.registry.Names(), "reflect", "complete_objective", "wait")
		valid := false
		for _, a := range allowed {
			if a == action.Action {
				valid = true
			}
		}
		if !valid {
			action.Action = "wait"
		}
		e.db.ExecContext(ctx, "UPDATE iterations SET objective_snapshot=?, plan=?, reasoning_summary=? WHERE id=?", prompt, rawAI, string(action.Parameters), iterID)
		e.bus.Publish(events.Event{Type: "iteration.started", Payload: map[string]any{"objective_id": objectiveID, "run_id": runID, "iteration_id": iterID, "prompt": prompt, "ai_raw": rawAI, "action": action.Action, "params": string(action.Parameters)}})

		var result any
		var actErr string
		if action.Action != "complete_objective" && action.Action != "wait" && action.Action != "reflect" {
			e.log.Debug("executing tool", "action", action.Action, "params", string(action.Parameters))
			res, err := e.registry.Execute(ctx, action.Action, action.Parameters)
			if err != nil {
				actErr = err.Error()
				e.log.Error("tool failed", "action", action.Action, "err", actErr)
				e.db.ExecContext(ctx, "UPDATE iterations SET action=?, action_result=?, reflection=?, status='failed', completed_at=CURRENT_TIMESTAMP WHERE id=?", action.Action, actErr, action.Reason, iterID)
				e.bus.Publish(events.Event{Type: "api.error", Payload: map[string]any{"objective_id": objectiveID, "run_id": runID, "iteration_id": iterID, "action": action.Action, "error": actErr}})
			} else {
				result = res
				b, _ := json.Marshal(result)
				e.log.Info("tool success", "action", action.Action, "result_len", len(b))
				e.handleResult(ctx, objectiveID, action.Action, action.Parameters, result)
				e.db.ExecContext(ctx, "UPDATE iterations SET action=?, action_result=?, reflection=?, status='completed', completed_at=CURRENT_TIMESTAMP WHERE id=?", action.Action, string(b), action.Reason, iterID)
				e.bus.Publish(events.Event{Type: "api.response", Payload: map[string]any{"objective_id": objectiveID, "run_id": runID, "iteration_id": iterID, "action": action.Action, "result": string(b)}})
			}
		} else if action.Action == "complete_objective" {
			e.db.ExecContext(ctx, "UPDATE objectives SET status='completed', completed_at=CURRENT_TIMESTAMP WHERE id=?", objectiveID)
			e.db.ExecContext(ctx, "UPDATE agent_runs SET status='completed', finished_at=CURRENT_TIMESTAMP WHERE id=?", runID)
			e.db.ExecContext(ctx, "UPDATE iterations SET action=?, reflection=?, status='completed', completed_at=CURRENT_TIMESTAMP WHERE id=?", action.Action, action.Reason, iterID)
			e.bus.Publish(events.Event{Type: "agent.completed", Payload: map[string]any{"objective_id": objectiveID}})
			return
		} else {
			e.db.ExecContext(ctx, "UPDATE iterations SET action=?, reflection=?, status='completed', completed_at=CURRENT_TIMESTAMP WHERE id=?", action.Action, action.Reason, iterID)
		}
		e.db.ExecContext(ctx, "UPDATE agent_runs SET iteration=? WHERE id=?", i+1, runID)
		e.db.ExecContext(ctx, "UPDATE objectives SET iteration_count=? WHERE id=?", i+1, objectiveID)
		if e.isComplete(ctx, objectiveID) {
			e.db.ExecContext(ctx, "UPDATE objectives SET status='completed', completed_at=CURRENT_TIMESTAMP WHERE id=?", objectiveID)
			e.db.ExecContext(ctx, "UPDATE agent_runs SET status='completed', finished_at=CURRENT_TIMESTAMP WHERE id=?", runID)
			e.bus.Publish(events.Event{Type: "agent.completed", Payload: map[string]any{"objective_id": objectiveID}})
			return
		}
		if actErr != "" {
			time.Sleep(1 * time.Second)
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}
	e.db.ExecContext(context.Background(), "UPDATE agent_runs SET status='failed', finished_at=CURRENT_TIMESTAMP WHERE id=?", runID)
	e.db.ExecContext(context.Background(), "UPDATE objectives SET status='failed' WHERE id=?", objectiveID)
}

func (e *Engine) plan(ctx context.Context, objectiveID int64) (Action, string, string) {
	var target *int
	var desc string
	var minScore int
	e.db.QueryRowContext(ctx, "SELECT target_leads, description, minimum_score FROM objectives WHERE id=?", objectiveID).Scan(&target, &desc, &minScore)
	var total, qualified int
	e.db.QueryRowContext(ctx, "SELECT count(*) FROM leads WHERE objective_id=?", objectiveID).Scan(&total)
	e.db.QueryRowContext(ctx, "SELECT count(*) FROM leads WHERE objective_id=? AND lead_score>=?", objectiveID, minScore).Scan(&qualified)
	recent := e.recentQueries(ctx, objectiveID)
	prompt := BuildPrompt(desc, target, total, qualified, minScore, e.registry.Describe(), recent)
	raw := ""
	if e.ai != nil {
		c2, cancel := context.WithTimeout(ctx, 30*time.Second)
		out, err := e.ai.Chat(c2, prompt, true)
		cancel()
		raw = out
		if err == nil {
			var a Action
			if json.Unmarshal([]byte(out), &a) == nil && a.Action != "" {
				return a, prompt, raw
			}
			raw = "AI error/invalid JSON: " + out + " err:" + err.Error()
		} else {
			raw = "AI timeout/fallback: " + err.Error()
		}
	}
	if target != nil && total < *target {
		q, cc, city, ind := DeriveSearchParams(desc)
		b, _ := json.Marshal(map[string]any{"q": q, "country_code": cc, "city": city, "industry": ind})
		if raw == "" {
			raw = "fallback heuristic (no AI)"
		}
		return Action{Action: "search_leads", Reason: "Need more leads", Parameters: b}, prompt, raw
	}
	return Action{Action: "complete_objective", Reason: "Target reached"}, prompt, raw
}

// recentQueries returns the last few search queries run for an objective so
// the planner can avoid repeating itself.
func (e *Engine) recentQueries(ctx context.Context, objectiveID int64) []string {
	rows, err := e.db.QueryContext(ctx, "SELECT query FROM search_history WHERE objective_id=? ORDER BY id DESC LIMIT 5", objectiveID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var q string
		if rows.Scan(&q) == nil {
			out = append(out, q)
		}
	}
	return out
}

func (e *Engine) handleResult(ctx context.Context, objectiveID int64, action string, params json.RawMessage, result any) {
	e.log.Debug("handleResult", "action", action, "objective", objectiveID)
	if action == "search_leads" {
		b, _ := json.Marshal(result)
		var leads []map[string]any
		json.Unmarshal(b, &leads)
		var pend []pendingLead
		for _, l := range leads {
			email, _ := l["email"].(string)
			if email == "" {
				continue
			}
			ext, _ := l["id"].(string)
			if ext == "" {
				if v, ok := l["ID"].(string); ok {
					ext = v
				}
			}
			// Providers occasionally return leads without an id; the email is
			// the natural dedupe key, otherwise every id-less lead after the
			// first is silently dropped by the UNIQUE(external_key) constraint.
			if ext == "" {
				ext = "email:" + email
			}
			fn, _ := l["first_name"].(string)
			if fn == "" {
				fn, _ = l["FirstName"].(string)
			}
			ln, _ := l["last_name"].(string)
			if ln == "" {
				ln, _ = l["LastName"].(string)
			}
			comp, _ := l["company_name"].(string)
			if comp == "" {
				comp, _ = l["Company"].(string)
			}
			dom, _ := l["company_domain"].(string)
			if dom == "" {
				dom, _ = l["Domain"].(string)
			}
			title, _ := l["position_title"].(string)
			if title == "" {
				title, _ = l["Title"].(string)
			}
			if title == "" {
				title, _ = l["q"].(string)
			}
			li, _ := l["linkedin_url"].(string)
			city, _ := l["city"].(string)
			cc, _ := l["country_code"].(string)
			ind, _ := l["industry_name"].(string)
			summary, _ := l["summary"].(string)
			if summary == "" {
				summary, _ = l["Summary"].(string)
			}
			loc := cc
			if loc == "" {
				loc = city
			}
			sc := scoring.ScoreWithSummary(title, ind, loc, "unchecked", dom, summary, []string{title}, []string{ind}, []string{cc, city})
			if summary != "" && len(summary) > 50 {
				sc.Score += 2
			}
			if strings.TrimSpace(summary) == "" {
				sc.Score = sc.Score - 5
				if sc.Score < 0 {
					sc.Score = 0
				}
			}
			_, _ = e.db.ExecContext(ctx, `INSERT OR IGNORE INTO leads(external_key,first_name,last_name,email,company_name,company_domain,position_title,linkedin_url,city,country_code,industry_name,summary,source,lead_score,score_breakdown,email_status,objective_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, ext, fn, ln, email, comp, dom, title, li, city, cc, ind, summary, "leadscaptain", sc.Score, sc.JSON(), "unchecked", objectiveID)
			// Skip re-verification when this email already carries a verdict;
			// verification calls cost money or rate budget on real providers.
			var existingStatus string
			e.db.QueryRowContext(ctx, "SELECT COALESCE(email_status,'') FROM leads WHERE email=? AND email_status IS NOT NULL AND email_status!='unchecked' LIMIT 1", email).Scan(&existingStatus)
			vparams, _ := json.Marshal(map[string]string{"email": email})
			if vr, err := e.registry.Execute(ctx, "verify_email", json.RawMessage(vparams)); err == nil && existingStatus == "" {
				bv, _ := json.Marshal(vr)
				var vm map[string]any
				json.Unmarshal(bv, &vm)
				st, _ := vm["Status"].(string)
				if st == "" {
					st, _ = vm["status"].(string)
				}
				if st != "" {
					e.db.ExecContext(ctx, `UPDATE leads SET email_status=? WHERE email=?`, st, email)
					if st == "valid" {
						e.db.ExecContext(ctx, `UPDATE leads SET lead_score=85 WHERE email=? AND lead_score<85`, email)
					} else if st == "invalid" {
						e.db.ExecContext(ctx, `UPDATE leads SET lead_score=10 WHERE email=?`, email)
					}
				}
			}
			pend = append(pend, pendingLead{Email: email, FirstName: fn, LastName: ln, Company: comp, Title: title, Summary: summary, City: city, Country: cc})
		}
		if len(leads) > 0 {
			// Record the actual query so the planner can avoid repeats and the
			// UI shows what was tried, not the literal string "search".
			qText := string(params)
			var sp map[string]any
			if json.Unmarshal(params, &sp) == nil {
				if qv, ok := sp["q"].(string); ok && qv != "" {
					qText = qv
					for _, k := range []string{"industry", "city", "country_code"} {
						if v, ok := sp[k].(string); ok && v != "" {
							qText += " " + v
						}
					}
				}
			}
			e.db.ExecContext(ctx, "INSERT INTO search_history(objective_id,query,result_count) VALUES(?,?,?)", objectiveID, qText, len(leads))
		}
		e.log.Info("leads inserted", "count", len(leads), "pend", len(pend))
		e.bus.Publish(events.Event{Type: "lead.created", Payload: map[string]any{"objective_id": objectiveID, "count": len(leads)}})
		if len(pend) > 0 {
			e.log.Info("generating intros", "count", len(pend))
			go e.generateIntros(pend)
		}
	}
}

func (e *Engine) generateIntros(leads []pendingLead) {
	sem := make(chan struct{}, 2)
	for _, ld := range leads {
		sem <- struct{}{}
		pl := ld
		go func() {
			defer func() { <-sem }()
			prompt := buildOutreachPrompt(pl)
			msg := ""
			if e.ai != nil {
				ctx2, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				out, err := e.ai.Chat(ctx2, prompt, false)
				cancel()
				if err == nil {
					msg = out
				}
			}
			if msg == "" {
				msg = "Hi " + pl.FirstName + ", I came across your profile at " + pl.Company + " and was impressed by your work as " + pl.Title + ". Would love to connect and share ideas."
			}
			if len(msg) > 500 {
				msg = msg[:500]
			}
			e.db.ExecContext(context.Background(), `UPDATE leads SET outreach_message=?, outreach_generated_at=CURRENT_TIMESTAMP WHERE email=?`, msg, pl.Email)
		}()
	}
}

func (e *Engine) isComplete(ctx context.Context, objectiveID int64) bool {
	var target *int
	var minScore int
	e.db.QueryRowContext(ctx, "SELECT target_leads, minimum_score FROM objectives WHERE id=?", objectiveID).Scan(&target, &minScore)
	if target == nil {
		return false
	}
	var qualified int
	e.db.QueryRowContext(ctx, "SELECT count(*) FROM leads WHERE lead_score>=? AND objective_id=?", minScore, objectiveID).Scan(&qualified)
	return qualified >= *target
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func itoa(n int) string { return json.Number(itoa2(n)).String() }
func itoa2(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func (e *Engine) Pause(ctx context.Context, objectiveID int64) error {
	_, err := e.db.ExecContext(ctx, "UPDATE objectives SET status='paused' WHERE id=?", objectiveID)
	return err
}

// lastIteration returns the highest iteration number recorded for a run, so
// a resumed loop continues after it instead of re-running or duplicating
// iterations. In-flight (interrupted) iterations count too: the planner
// re-decides every iteration, so skipping an interrupted plan loses nothing.
func (e *Engine) lastIteration(ctx context.Context, runID int64) int {
	var n sql.NullInt64
	e.db.QueryRowContext(ctx, "SELECT MAX(iteration_number) FROM iterations WHERE run_id=?", runID).Scan(&n)
	return int(n.Int64)
}

func (e *Engine) Resume(ctx context.Context, objectiveID int64) error {
	var objStatus string
	if err := e.db.QueryRowContext(ctx, "SELECT status FROM objectives WHERE id=?", objectiveID).Scan(&objStatus); err != nil {
		return fmt.Errorf("objective %d: %w", objectiveID, err)
	}
	if objStatus != "paused" {
		return fmt.Errorf("objective %d is %s, not paused", objectiveID, objStatus)
	}
	var runID int64
	if err := e.db.QueryRowContext(ctx, "SELECT id FROM agent_runs WHERE objective_id=? ORDER BY id DESC LIMIT 1", objectiveID).Scan(&runID); err != nil {
		return fmt.Errorf("objective %d has no run to resume: %w", objectiveID, err)
	}
	iter := e.lastIteration(ctx, runID)
	if _, err := e.db.ExecContext(ctx, "UPDATE objectives SET status='running' WHERE id=?", objectiveID); err != nil {
		return err
	}
	e.db.ExecContext(ctx, "UPDATE agent_runs SET status='running' WHERE id=?", runID)
	go e.loop(runID, objectiveID, iter)
	return nil
}
func (e *Engine) Stop(ctx context.Context, objectiveID int64) error {
	_, err := e.db.ExecContext(ctx, "UPDATE objectives SET status='stopped' WHERE id=?", objectiveID)
	return err
}
func (e *Engine) Recover(ctx context.Context) {
	rows, _ := e.db.QueryContext(ctx, "SELECT id, objective_id FROM agent_runs WHERE status='running'")
	if rows == nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var runID, objID int64
		rows.Scan(&runID, &objID)
		go e.loop(runID, objID, e.lastIteration(ctx, runID))
	}
}
