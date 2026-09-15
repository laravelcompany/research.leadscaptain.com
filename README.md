# Autonomous Lead Research Agent
Repository: https://github.com/laravelcompany/research.leadscaptain.com
(moved from `izdrail/research-leads.izdrail.com` - old URLs redirect here)

Go + React lead research platform. An agent engine plans iterations with an
LLM, calls tools, scores and stores leads, and drafts outreach - all over a
chi REST API with a React frontend and a SQLite store.

## Agent tools
The planner chooses one tool per iteration from the live registry (the prompt
is built from the registry, so tools and docs never drift):

| Tool | Purpose | Key required? |
| --- | --- | --- |
| `search_leads` | Search `api.leadscaptain.com` | LeadsCaptain token |
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

## UI auth (login screen)
Set `AUTH_USERNAME` and `AUTH_PASSWORD` in `.env` and the web UI shows a login
screen before any data loads. Login issues a signed HttpOnly session cookie
(7-day expiry); the header gains a Sign out button. With either variable
empty the login screen is disabled and the app behaves as before.

- `POST /api/v1/auth/login` `{"username":"...","password":"..."}` - logs in,
  sets the session cookie. Also returns the cookie for browser use.
- `GET /api/v1/auth/me` - `{"auth_required":bool,"authenticated":bool}`.
- `POST /api/v1/auth/logout` - clears the cookie.
- API clients keep using `Authorization: Bearer $APP_API_KEY` untouched; a
  valid UI session cookie also satisfies the `APP_API_KEY` gate so the UI
  works when both are configured. Health and metrics endpoints stay open.
- `AUTH_SESSION_SECRET` optionally overrides the cookie-signing key (default
  is derived from the credentials, so sessions survive restarts but are
  invalidated when credentials change).

```bash
curl -c cookies.txt -X POST localhost:7001/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"S3curePass!"}'
curl -b cookies.txt localhost:7001/api/v1/stats
curl -b cookies.txt -X POST localhost:7001/api/v1/auth/logout
```

## Quick start
```bash
cp .env.example .env
make build
make dev

# Frontend
cd frontend && npm install && npm run dev

## Docker
docker compose up --build

# or use the published image (CI pushes on main and v* tags)
docker pull izdrail/research.leadscaptain.com:latest
docker run -p 7001:7001 --env-file .env izdrail/research.leadscaptain.com:latest
```

The local SQLite database is disposable working state, not the lead source.
By default it lives at `/tmp/research-leads/leads.db`, and CSV exports live
under `/tmp/research-leads/exports`; both start fresh when the container is
replaced. Do not attach persistent storage for these paths in Coolify.

Lead search uses `https://api.leadscaptain.com/leads`. Set
`LEADSCAPTAIN_API_TOKEN` to the token issued by LeadsCaptain. The client sends
both supported auth forms (`Authorization: Bearer ...` and `X-API-Token`) and
maps the documented `q`, `position_title`, `country_code`, `location`,
`industry_name`, `page`, and `limit` parameters:

```env
LEADSCAPTAIN_BASE_URL=https://api.leadscaptain.com
LEADSCAPTAIN_API_TOKEN=your-token
DATABASE_PATH=/tmp/research-leads/leads.db
EXPORT_DIR=/tmp/research-leads/exports
```

## Notable behavior
- Objectives have `target_leads` / `minimum_score`; the agent stops when the
  target is met for *that objective's* leads.
- Starting an already-running objective is rejected; pause/resume continues
  from the last completed iteration.
- `POST /api/v1/imports` (multipart `file`) imports a CSV of leads, deduped
  by email. `GET /api/v1/leads/export` exports CSV.
- `POST /api/v1/leads/search` runs a whitelisted filter query
  (`{"filters":[{"field":"title","operator":"CONTAINS","value":"CTO"}]}`).
- Lead search requires `LEADSCAPTAIN_API_TOKEN`; the base URL defaults to
  `https://api.leadscaptain.com`. Email validation keeps its deterministic
  local fallback when `EMAIL_VALIDATION_URL` is unset.
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
