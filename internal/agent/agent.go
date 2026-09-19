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
	consecutiveToolFailures := 0
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
				consecutiveToolFailures++
				e.log.Error("tool failed", "action", action.Action, "err", actErr, "consecutive_failures", consecutiveToolFailures)
				e.db.ExecContext(ctx, "UPDATE iterations SET action=?, action_result=?, reflection=?, status='failed', completed_at=CURRENT_TIMESTAMP WHERE id=?", action.Action, actErr, action.Reason, iterID)
				e.bus.Publish(events.Event{Type: "api.error", Payload: map[string]any{"objective_id": objectiveID, "run_id": runID, "iteration_id": iterID, "action": action.Action, "error": actErr}})
				if consecutiveToolFailures >= 3 {
					e.failRun(ctx, runID, objectiveID, fmt.Sprintf("%s failed %d consecutive times: %s", action.Action, consecutiveToolFailures, actErr))
					return
				}
			} else {
				consecutiveToolFailures = 0
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

func (e *Engine) failRun(ctx context.Context, runID, objectiveID int64, reason string) {
	e.db.ExecContext(ctx, "UPDATE agent_runs SET status='failed', finished_at=CURRENT_TIMESTAMP WHERE id=? AND status='running'", runID)
	e.db.ExecContext(ctx, "UPDATE objectives SET status='failed' WHERE id=? AND status='running'", objectiveID)
	e.bus.Publish(events.Event{Type: "agent.failed", Payload: map[string]any{"objective_id": objectiveID, "run_id": runID, "error": reason}})
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
			// Check the company site is still up before verifying the email:
			// lead found -> check website -> check email -> score. A verdict
			// already stored for this domain is reused, matching the email
			// reuse below, so one batch costs one fetch per domain.
			websiteStatus, freshSite := "unchecked", false
			if dom != "" {
				var existingSite string
				e.db.QueryRowContext(ctx, "SELECT COALESCE(website_status,'') FROM leads WHERE company_domain=? AND website_status IS NOT NULL AND website_status!='' AND website_status!='unchecked' LIMIT 1", dom).Scan(&existingSite)
				if existingSite != "" {
					websiteStatus = existingSite
				} else {
					wparams, _ := json.Marshal(map[string]string{"domain": dom})
					if wr, err := e.registry.Execute(ctx, "check_website", json.RawMessage(wparams)); err == nil {
						bw, _ := json.Marshal(wr)
						var wm map[string]any
						if json.Unmarshal(bw, &wm) == nil && len(wm) > 0 {
							websiteStatus = classifyWebsite(wm)
							freshSite = true
						}
					}
				}
			}
			// Verify the address before scoring so the stored status and score
			// reflect a real check instead of the "unchecked" placeholder. A
			// verdict already stored for this email is reused: verification
			// calls cost money or rate budget on real providers.
			emailStatus, freshVerdict := "unchecked", ""
			if email != "" {
				var existingStatus string
				e.db.QueryRowContext(ctx, "SELECT COALESCE(email_status,'') FROM leads WHERE email=? AND email_status IS NOT NULL AND email_status!='unchecked' LIMIT 1", email).Scan(&existingStatus)
				if existingStatus != "" {
					emailStatus = existingStatus
				} else {
					vparams, _ := json.Marshal(map[string]string{"email": email})
					if vr, err := e.registry.Execute(ctx, "verify_email", json.RawMessage(vparams)); err == nil {
						bv, _ := json.Marshal(vr)
						var vm map[string]any
						json.Unmarshal(bv, &vm)
						st, _ := vm["Status"].(string)
						if st == "" {
							st, _ = vm["status"].(string)
						}
						if st != "" {
							emailStatus, freshVerdict = st, st
						}
					}
				}
			}
			sc := scoring.ScoreWithSummary(title, ind, loc, emailStatus, dom, websiteStatus, summary, []string{title}, []string{ind}, []string{cc, city})
			sc.Score = adjustForSummary(sc.Score, summary)
			_, _ = e.db.ExecContext(ctx, `INSERT OR IGNORE INTO leads(external_key,first_name,last_name,email,company_name,company_domain,position_title,linkedin_url,city,country_code,industry_name,summary,source,lead_score,score_breakdown,email_status,website_status,objective_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, ext, fn, ln, email, comp, dom, title, li, city, cc, ind, summary, "leadscaptain", sc.Score, sc.JSON(), emailStatus, websiteStatus, objectiveID)
			// Propagate fresh verdicts to every other row carrying this
			// address or domain and rescore them with the standard scorer -
			// never hard-coded values - so the breakdown stays auditable.
			if freshVerdict != "" {
				e.rescoreLeads(ctx, "email=?", email, freshVerdict, "")
			}
			if freshSite && websiteStatus != "unchecked" {
				e.rescoreLeads(ctx, "company_domain=? AND (website_status IS NULL OR website_status='' OR website_status='unchecked')", dom, "", websiteStatus)
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

// adjustForSummary applies the ingestion-time summary tweaks shared by the
// initial insert and the post-verification rescore so both agree on a score.
func adjustForSummary(score int, summary string) int {
	if summary != "" && len(summary) > 50 {
		score += 2
	}
	if strings.TrimSpace(summary) == "" {
		score -= 5
		if score < 0 {
			score = 0
		}
	}
	return score
}

// classifyWebsite maps a check_website tool result to the stored website
// status: live, parked, error (HTTP >= 400), unreachable, or unchecked when
// the tool returned nothing usable.
func classifyWebsite(result map[string]any) string {
	reachable, _ := result["reachable"].(bool)
	if !reachable {
		return "unreachable"
	}
	if parked, _ := result["parked"].(bool); parked {
		return "parked"
	}
	if code, _ := result["status_code"].(float64); code >= 400 {
		return "error"
	}
	return "live"
}

// rescoreLeads propagates fresh verdicts to the leads matched by where and
// recomputes each row's score with the standard scorer. A non-empty
// emailStatus or websiteStatus overwrites the row's stored value (and stamps
// its verified/checked time); an empty one keeps the row's value. Verification
// moves the score through the same breakdown as every other factor, and drift
// is recorded in lead_score_history.
func (e *Engine) rescoreLeads(ctx context.Context, where string, arg any, emailStatus, websiteStatus string) {
	rows, err := e.db.QueryContext(ctx, `SELECT id,COALESCE(position_title,''),COALESCE(industry_name,''),COALESCE(country_code,''),COALESCE(city,''),COALESCE(company_domain,''),COALESCE(summary,''),COALESCE(email_status,''),COALESCE(website_status,''),lead_score FROM leads WHERE `+where, arg)
	if err != nil {
		return
	}
	// Buffer before writing: sqlite runs on one connection, so updating while
	// the read is still open would deadlock.
	type leadRow struct {
		id                                 int64
		title, ind, cc, city, dom, summary string
		es, ws                             string
		oldScore                           int
	}
	var batch []leadRow
	for rows.Next() {
		var r leadRow
		if rows.Scan(&r.id, &r.title, &r.ind, &r.cc, &r.city, &r.dom, &r.summary, &r.es, &r.ws, &r.oldScore) == nil {
			batch = append(batch, r)
		}
	}
	rows.Close()
	for _, row := range batch {
		id, title, ind, cc, city, dom, summary, es, ws, oldScore := row.id, row.title, row.ind, row.cc, row.city, row.dom, row.summary, row.es, row.ws, row.oldScore
		if emailStatus != "" {
			es = emailStatus
		}
		if websiteStatus != "" {
			ws = websiteStatus
		}
		loc := cc
		if loc == "" {
			loc = city
		}
		r := scoring.ScoreWithSummary(title, ind, loc, es, dom, ws, summary, []string{title}, []string{ind}, []string{cc, city})
		r.Score = adjustForSummary(r.Score, summary)
		q := `UPDATE leads SET email_status=?, website_status=?, lead_score=?, score_breakdown=?, updated_at=CURRENT_TIMESTAMP`
		if emailStatus != "" {
			q += `, email_verified_at=CURRENT_TIMESTAMP`
		}
		if websiteStatus != "" {
			q += `, website_checked_at=CURRENT_TIMESTAMP`
		}
		q += ` WHERE id=?`
		if _, err := e.db.ExecContext(ctx, q, es, ws, r.Score, r.JSON(), id); err != nil {
			continue
		}
		if oldScore != r.Score {
			e.db.ExecContext(ctx, "INSERT INTO lead_score_history(lead_id,score_type,old_score,new_score,breakdown) VALUES(?,?,?,?,?)", id, "lead", oldScore, r.Score, r.JSON())
		}
	}
}
