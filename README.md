# Autonomous Lead Research Agent
Go + React lead research platform. An agent engine plans iterations with an
LLM, calls tools, scores and stores leads, and drafts outreach - all over a
chi REST API with a React frontend and a SQLite store.

## Agent tools
The planner chooses one tool per iteration from the live registry (the prompt
is built from the registry, so tools and docs never drift):

| Tool | Purpose | Key required? |
| --- | --- | --- |
| `search_leads` | Search LeadsCaptain (mock offline) | LeadsCaptain token |
| `verify_email` | Verify one address | validator key |
| `find_email` | Generate + verify common patterns for a person at a domain | no |
| `get_lead` / `list_leads` | Inspect stored leads | no |
| `score_lead` | Recompute + persist a lead's score, with history | no |
| `export_leads` | Write filtered leads to CSV under `EXPORT_DIR` | no |
| `check_website` | Site liveness, title, description, tech fingerprints | no |
| `check_domain` | MX / SPF / DMARC posture + mail-provider guess | no |
| `company_lookup` | European company registries, routed by country | see below |
| `get_statistics` | Aggregate counts | no |

Plus control actions `reflect`, `complete_objective`, `wait`.

### company_lookup routes
| Input | Provider | Key? |
| --- | --- | --- |
| `{"name":"...","country":"GB"}` | Companies House Public Data API | free key (`COMPANIES_HOUSE_API_KEY`) |
| `{"name":"...","country":"FR"}` | recherche-entreprises.api.gouv.fr | none |
| `{"fiscal_code":"...","country":"RO"}` | ANAF PlatitorTvaRest (by CUI) | none |
| `{"vat_number":"DE..."}` | VIES (any EU member state + XI) | none |

Unsupported routes fail safe with an actionable error (e.g. DE name search:
pass a VAT number for VIES instead) - the stream and other tools are never
affected.

## Planner
`internal/ai/prompts/planner.txt` holds the planning instructions; the engine
appends the objective, live progress, the registry tool list and the searches
already run (so the agent does not repeat itself). Without an AI endpoint the
fallback heuristic derives the search query, country and city from the
objective text instead of hardcoded defaults.

## Quick start
cp .env.example .env
make build
make dev

# Frontend
cd frontend && npm install && npm run dev

## Docker
docker compose up --build

## Notable behavior
- Objectives have `target_leads` / `minimum_score`; the agent stops when the
  target is met for *that objective's* leads.
- Starting an already-running objective is rejected; pause/resume continues
  from the last completed iteration.
- `POST /api/v1/imports` (multipart `file`) imports a CSV of leads, deduped
  by email. `GET /api/v1/leads/export` exports CSV.
- `POST /api/v1/leads/search` runs a whitelisted filter query
  (`{"filters":[{"field":"title","operator":"CONTAINS","value":"CTO"}]}`).
- Without `LEADSCAPTAIN_BASE_URL` / `EMAIL_VALIDATION_URL` the server uses
  deterministic mock providers for local development.
- Id-less leads from a provider dedupe by email (fallback external key), and
  addresses with an existing verdict are not re-verified.
- `company_lookup` fails safe with a clear error when a route is not
  available (missing GB key, unsupported country); everything else keeps
  working.
- `CORS_ALLOWED_ORIGINS` is honored when set (comma-separated); defaults to `*`.

## Tests
go test ./...

## Docs
- docs/architecture.md
- docs/patents/lead-research-patent-landscape.md
