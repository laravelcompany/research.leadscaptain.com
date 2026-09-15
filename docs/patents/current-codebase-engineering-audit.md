# Autonomous lead research agent: current codebase engineering audit

**Audit date:** 15 September 2026  
**Repository state inspected:** `laravelcompany/research.leadscaptain.com` `main` at `9205ad7`  
**Purpose:** establish what the running code implements, what is partial, what is schema or roadmap only, and which improvements fit the current architecture.

This is an engineering assessment, not legal advice. It does not state that an implementation is legally safe, infringing, or non-infringing. Patent counsel should review any freedom-to-operate question.

## Executive result

The product is a persisted, tool-using lead-research loop, not a predictive lead-scoring or CRM reconciliation platform. Its strongest implemented path is objective execution: an objective creates a run, the planner selects one registered tool per iteration, results are persisted, leads are ingested, emails may be verified, outreach text is generated, and progress is exposed through APIs and SSE events.

Lead scoring is deterministic attribute/rule arithmetic. It has no vendor benchmark normalisation, clustering, training set, learned model, feature selection, model selection, sampling, conversion feedback, or iterative model optimisation. Lead ordering is an internal query/UI concern, not an auction, exchange, marketplace, bidding, allocation, or buyer-routing system.

Several database tables suggest broader ambitions, but schema is not implementation. `lead_merges`, `lead_score_history`, `lead_enrichments`, `research_strategies`, `api_requests`, `ai_usage`, and `audit_log` are created but most are not written by the active execution path. Deduplication is exact-key suppression, not entity resolution. Email handling delegates validation to a configured provider. Outreach uses an LLM and stored lead fields, but before this audit it interpolated untrusted provider text directly into the instruction prompt.

The code change accompanying this audit addresses that concrete trust-boundary flaw only. Lead data is now bounded, JSON encoded, explicitly labelled untrusted data, and separated from the outreach instruction. It deliberately does not add a fact graph, ML scoring, outcome learning, fuzzy golden records, mailbox probing, or correction suggestions merely because those concepts appear in the landscape.

## Method and evidence boundary

The audit followed calls and data flow rather than relying on names:

1. HTTP composition and route registration in `internal/api/routes.go` and `internal/api/handlers/handlers.go`.
2. Objective lifecycle and planner loop in `internal/agent/agent.go` and `internal/agent/planner_prompt.go`.
3. Registered tool implementations in `internal/tools`, including lead search, scoring, verification, website/domain checks, company lookup and export.
4. Persistence schema and migrations in `internal/db/migrations`, plus actual SQL call sites.
5. External clients in `internal/leadscaptain`, `internal/emailvalidator`, `internal/companyreg`, `internal/webcheck`, and `internal/ai`.
6. Frontend API, lead, objective and SSE paths in `frontend/src`.
7. Tests under `internal/**/_test.go` and production builds.

The repository already contains the supplied technical landscape at `docs/patents/lead-research-patent-landscape.md`. It records a prior search of `izdrail/maicoach/research/patents/library` and wider Google Patents sources. This audit treats that document as the comparison brief and validates only this repository's engineering facts. It does not independently interpret patent claims.

### Classification vocabulary

- **Implemented:** an active code path is wired, persisted or exposed, and testable.
- **Partial:** a narrower implementation exists, or the path omits material reliability or product pieces.
- **Schema/planned only:** tables, fields or documentation exist without an active end-to-end path.
- **Missing:** no implementation was found.
- **Materially different:** nearby functionality exists, but the architecture described in the research is absent.

## Current architecture and end-to-end flow

### Objective execution and agentic planning: implemented, with limits

`POST /api/v1/objectives/{id}/start` calls `Engine.Start`. It validates the objective, creates an `agent_runs` row, marks the objective running, publishes `agent.started`, and starts one in-process goroutine. `Engine.loop` persists an `iterations` row, builds a stateful prompt from the objective, counts, available tools and recent searches, calls the configured OpenAI-compatible chat endpoint, parses one JSON action, invokes the registered tool, stores the result/reflection, updates counters, and stops when the target or iteration limit is reached.

Pause, resume, stop and startup recovery exist. A generation counter prevents an old loop continuing beside a resumed loop. Interrupted iterations are marked honestly. This is autonomous action selection over a fixed tool registry, but it is not recursive plan decomposition, a durable distributed job worker, or a self-modifying learner. The `tasks` and `jobs` tables are not the core loop's execution queue.

### Discovery and ingestion: implemented, with provider dependence

`search_leads` delegates to `internal/leadscaptain.Client`. `Engine.handleResult` normalises provider field variants, skips records without email, creates a fallback external key from email, computes a rule score, inserts with `INSERT OR IGNORE`, records search history, triggers verification, and asynchronously generates outreach. Discovery is therefore API search plus local normalisation, not a general web crawler.

The code has free company-registry, website, DNS/domain and email tools that the planner can choose. Those enrich or qualify a lead when explicitly called; there is no universal enrichment pipeline that automatically reconciles all provider results.

### Persistence: implemented, broader than active behaviour

SQLite stores objectives, runs, iterations, leads, searches, imports and bulk-operation state. Migrations are embedded and applied in order. Objective attribution was added to leads in migration `007_objective_leads.sql`.

Tables for audit, API requests, AI usage, findings, merge records, score history, enrichment history and strategy performance exist. Searches of active SQL call sites found little or no production writing for several of these. Their existence must not be reported as finished observability, learning, reconciliation, or provenance.

## Capability inventory

| Capability | Classification | What actually exists | Important gaps |
|---|---|---|---|
| Agentic planning loop | Implemented | One-action-per-iteration LLM planner, fixed tools, persisted runs/iterations, recent-query context, completion checks, pause/resume/stop/recovery | In-process only; no durable worker lease; no learned policy; limited structured planning |
| Lead discovery | Implemented | LeadsCaptain API search plus planner-selected company/domain/website tools | No autonomous browser/crawler path; provider coverage and freshness are external |
| Lead ingestion | Implemented | Field normalisation, raw provider result handling, objective attribution, exact duplicate suppression | Records without email are discarded; failures are often logged rather than surfaced transactionally |
| Lead scoring | Implemented but rule-based | Additive title/industry/location/email/domain/summary rules, clamped 0-100, JSON breakdown; separate unused-ish ICP/data-quality/intent/engagement helpers | Ingestion passes observed values as their own targets, so match bonuses are weakly objective-specific; verification later overwrites score with 85/10 instead of recomputing the same breakdown; no score-method/version field |
| Email discovery | Partial | `find_email` tool is registered and can call the configured lead source | No internet-search-and-candidate-generation implementation in this repo; no evidence/confidence trail |
| Email verification | Implemented by delegation | Configured verifier client, result status, timestamp fields, cached verdict reuse, bulk verify operation | No local SMTP/mailbox probing, typo correction, campaign response tracking, or provider evidence payload persistence |
| Deduplication | Partial | Unique `external_key`, email fallback key, `INSERT OR IGNORE`, case-lowered within-file CSV duplicate detection and DB email existence checks | No canonical email normalisation across every path; no fuzzy match, confidence, survivorship rules, golden record, review queue or merge/unmerge service |
| Entity resolution | Schema/planned only | `lead_merges`, `companies`, relationships and enrichment tables exist | No active matcher, merge writer, conflict resolver, human review UI or audit reconstruction |
| Outreach generation | Implemented, now hardened | Async LLM generation from selected stored lead fields with deterministic fallback and stored timestamp | No send operation, approval flow, campaign result tracking or validity-window fact graph; async failures are not persisted distinctly |
| CSV import | Implemented | Multipart CSV parser, alias mapping, required email, per-file and DB duplicate counts, import record | No objective selection, dry run, row-level error report, transactional all-or-nothing mode or advanced mapping UI |
| CSV export | Implemented | API CSV export and agent `export_leads` tool | Current-main API export is global; objective-scoped export is in pending PR #9, not `main` |
| Objective execution | Implemented | CRUD, lifecycle actions, counts, run/iteration/search state | Deleting an objective does not clearly remove/reassign its leads; several global endpoints ignore objective scope on current `main` |
| Events | Partial on current `main` | In-memory fan-out bus, timestamped JSON SSE, bounded subscriber channel, unsubscribe on disconnect, frontend reconnect/backoff | Not durable; dropped silently for slow clients; many event payloads lack objective ID on current `main`; no resume cursor or replay |
| Audit/debug | Partial/schema-heavy | Iterations, search history, SSE panel, audit endpoint/table, basic logs and metrics | `audit_log`, `api_requests` and `ai_usage` are not consistently populated; no correlation IDs, durable event stream or secret/PII redaction policy |
| API integrations | Implemented | OpenAI-compatible AI, LeadsCaptain, email verifier, registries, VIES/company sources, website and DNS checks | Provider reliability varies; telemetry is incomplete; some result handling ignores errors |
| Frontend | Implemented for global workflow | Objectives, global leads, runs, iterations, event stream, auth, loading/empty components | On audited `main`, objective cards do not yet provide the dedicated isolated workflow described in PR #9 |

## Patent-area comparison

### US11775912B2: real-time lead grading

**Classification: materially different, with generic attribute-scoring overlap.**

The active scorer is `internal/scoring/scorer.go`. It adds fixed points for title, industry and location string matches, verified email and company domain, subtracts fixed points for invalid/generic email, then adds summary length/content bonuses. `internal/leads/score.go` contains another fixed weighted helper (`ICP`, `DataQuality`, `Intent`, `Engagement`) but the active ingestion path uses `scoring.ScoreWithSummary`.

There is no vendor-population benchmark, percentile or benchmark-value conversion. There are no pre-determined clusters, cluster assignment, cohort statistics or cluster-specific grade. Scoring happens near ingestion and can be recalculated, so it is "real-time" only in the ordinary engineering sense. That should not be conflated with the complete mechanism described in the landscape.

**Engineering decision:** do not add benchmark normalisation or clustering without a separate product need and design review. First fix internal consistency: derive targets from structured objective criteria, recompute after verification instead of forcing 85/10, and persist scorer version/method with score history.

### US11544582B2: predictive lead scoring

**Classification: missing and intentionally not introduced.**

No training dataset, labels, train/test split, model artefact, model registry, inference runtime, automated variable selection, feature-set selection, training-data sampling, local/global search or iterative optimisation exists. LLM action planning is not a predictive scoring model. `research_strategies` counters are schema only and do not train a scorer.

**Engineering decision:** accurately call the system rule-based. Do not add ML to imitate the landscape. If predictive scoring becomes a product requirement, begin with data contracts, outcome quality, offline evaluation, drift monitoring, explainability and human controls before model automation.

### US10394834B1 and US10860593: ranking leads based on characteristics

**Classification: materially different.**

Lead scores support minimum-score qualification and SQL/UI ordering. Advanced search permits controlled sorting. There is no seller/buyer marketplace, auction, exchange, appraisal for a bid, bidding, clearing, allocation or routing between purchasers. Generic internal ranking is the full present scope.

**Engineering decision:** keep ordinary sortable lead lists. Do not describe this as a marketplace ranking implementation.

### US20140149178A1: outcome-feedback lead scoring

**Classification: missing.**

The database stores lead lifecycle status and status history, but no active job joins realised outcomes back to score parameters. The scorer's weights are constants. No conversion likelihood is estimated, and no score model is updated from won/lost/replied/booked outcomes. Recent search history prevents repeated queries; it is not model feedback.

**Engineering decision:** do not silently turn operational status data into automated score changes. A useful precursor is explicit outcome taxonomy, provenance, immutable score history and offline reports comparing score bands to outcomes. Any adaptive loop needs a separate technical and legal review.

### US8495151B2: determining missing email addresses

**Classification: partial by external delegation, not locally implemented.**

The planner exposes `find_email`, and a provider may return candidate contact data. The repository does not crawl the internet from partial contact details or locally generate permutations and validate candidates. Ingestion currently skips leads with no email, making missing-email enrichment less useful than the planner text implies.

**Engineering decision:** improve provider-result provenance, confidence and explicit candidate status before broadening discovery. Avoid building a hidden scraping/permutation pipeline as an incidental change.

### US9917806B2: erroneous-address detection and corrected-address recommendation

**Classification: materially different.**

`internal/emailvalidator.Client` sends an address to a configured verification API and records status/reason. It does not detect a mistaken intended recipient and propose a corrected address. There is no "did you mean" flow.

**Engineering decision:** keep verify-only behaviour unless correction is independently required. Persist verifier/provider evidence and distinguish `unknown`, `risky`, `catch_all`, `valid` and `invalid` consistently.

### DE202026101185U1: agent-based reconciliation and golden records

**Classification: partial exact deduplication; golden-record architecture missing.**

Exact controls exist at ingestion and import. A `lead_merges` table exists, but no production path creates or consumes merge records. There is no fuzzy entity matcher, confidence model, field-level source lineage, survivorship policy, golden-record materialisation, review console, human resolution or continuous learning from merge decisions.

**Engineering decision:** do not label exact duplicate suppression "entity resolution." A pragmatic next step would be a read-only duplicate-candidate report using conservative deterministic keys, followed by a reviewed merge design with source lineage and undo. It should not be added casually to the autonomous loop.

### CN122066453A: grounded marketing generation

**Classification: partial; no fact graph.**

`generateIntros` builds outreach from lead fields and stores the result. Before this audit, external summary, company and title text were concatenated directly into the instruction. There was no explicit trust boundary, source selection, claim validation, fact graph, validity window or citation record.

This audit adds an architecture-fitting safety improvement: selected lead fields are trimmed, control characters removed, rune-bounded, JSON encoded under `LEAD_DATA_JSON`, and preceded by an explicit instruction that the payload is untrusted data and cannot change the task. Tests verify delimiter integrity, JSON quoting and Unicode-safe bounds. This is simple prompt/data separation, not a marketing fact-map implementation.

**Further useful work:** store which lead fields and source versions grounded each generated message, persist generation failure, and allow regeneration only from reviewed fields. Do not invent claims.

### KR20260070355A: web-agent manifests and prompt-injection trust boundaries

**Classification: the page-manifest mechanism is missing; the trust-boundary concern is relevant.**

The system has website and domain checks, not an autonomous DOM-browsing agent that consumes embedded workflow manifests. No trust registry for webpage-authored agent instructions exists. Provider summaries and future web content are nevertheless untrusted inputs.

The outreach hardening in this change applies the minimum relevant principle: external text is data, not authority. If browser research is later added, fetched content must not choose tools, broaden objectives, disclose data, or initiate side effects. Tool permissions and allowed domains should come from the objective/configuration, not the page.

## Objective isolation and pending PR #9

The audit baseline is current `main` (`9205ad7`). Branch `feat/objective-leads-workflow` at `132e55b` and PR #9 were not merged at audit time. That pending branch adds an objective route, scoped lead API filters/export, detailed lead drawer and objective-filtered SSE/debug UI. This audit does not duplicate those changes.

On current `main`, `objective_id` is stored and completion counts are objective-specific in the agent, but the principal leads list/export and event stream are global. Some emitted `lead.created`, iteration and API events omit `objective_id`. Therefore end-to-end objective isolation is **partial on main**, even though the data model can support it. Rebase this audit after PR #9 if it merges first, and retain PR #9's isolation tests.

## Reliability and security findings

### High priority

1. **Unify score calculation.** Initial scoring produces a breakdown, while verification forces score 85 or 10. Recalculate through one service and append `lead_score_history` with method/version.
2. **Use actual objective criteria.** Ingestion currently passes each observed title/industry/location back as its own target, making several matches tautological. Add structured objective criteria or a deterministic parser with explicit limitations.
3. **Make observability real.** Write `api_requests`, `ai_usage` and `audit_log` from central client/middleware boundaries, with objective/run/iteration correlation and redaction.
4. **Preserve objective isolation.** Merge/test PR #9, then ensure all writes, reads, exports, events and deletes have explicit scope rules.
5. **Handle ingestion transactionally enough to explain losses.** Record skipped no-email records, insert conflicts, verification failures and outreach failures rather than silently continuing.

### Medium priority

- Normalise emails and domains once at boundaries; add deliberate unique-key policy rather than relying on provider keys and ad hoc queries.
- Give SSE events stable IDs and durable replay if debugging across disconnects matters.
- Replace fire-and-forget outreach goroutines with bounded tracked work tied to run lifecycle.
- Add row-level import errors, dry run and optional objective attribution.
- Decide whether unused schema is roadmap or dead weight; document or remove it in a migration-safe release.

## Change implemented in this audit

Files:

- `internal/agent/outreach_prompt.go`
- `internal/agent/outreach_prompt_test.go`
- `internal/agent/agent.go`

Behaviour:

- central builder separates fixed instructions from external lead data;
- payload is valid JSON, which prevents quotes/newlines from changing prompt structure;
- external fields are trimmed, control-character cleaned and bounded;
- prompt explicitly says external content cannot supply instructions, links, role changes, tool directions or new facts;
- existing fallback message, storage path and architecture remain unchanged.

This reduces accidental prompt-control risk. It is not a proof against all prompt injection, does not validate facts, and does not create a patent-like fact graph or trust registry.

## Validation and acceptance criteria

Run from repository root:

```bash
go vet ./...
go test ./...
go build ./cmd/server
cd frontend && npm ci && npm run build
```

The change is backend prompt construction only and has no visual output change, so a new browser screenshot would not validate it. PR #9 carries separate visual checks for its UI work.

## Source notes

- Repository landscape brief: `docs/patents/lead-research-patent-landscape.md`
- US11775912B2: https://patents.google.com/patent/US11775912B2/en
- US11544582B2: https://patents.google.com/patent/US11544582B2/en
- US10394834B1: https://patents.google.com/patent/US10394834B1/en
- US10860593: https://patents.google.com/patent/US10860593/en
- US20140149178A1: https://patents.google.com/patent/US20140149178A1/en
- US8495151B2: https://patents.google.com/patent/US8495151B2/en
- US9917806B2: https://patents.google.com/patent/US9917806B2/en
- DE202026101185U1: https://patents.google.com/patent/DE202026101185U1/en
- CN122066453A: https://patents.google.com/patent/CN122066453A/en
- KR20260070355A: https://patents.google.com/patent/KR20260070355A/en

Patent summaries above are scoped to the supplied landscape. The codebase classifications are based on direct source inspection at the stated commit.
