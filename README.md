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
| `verify_email` | Verify one address: syntax, disposable/role, MX, SMTP probe + catch-all | no (external service optional) |
| `find_email` | Generate + verify common patterns for a person at a domain | no |
| `get_lead` / `list_leads` | Inspect stored leads | no |
| `score_lead` | Recompute + persist a lead's score, with history | no |
| `export_leads` | Write filtered leads to CSV under `EXPORT_DIR` | no |
| `check_website` | Site liveness, parked/for-sale detection, title, description, tech fingerprints | no |
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

## LinkedIn authentication

The browser UI has one authentication path: **Sign in with LinkedIn using OpenID Connect**. The old `AUTH_USERNAME` / `AUTH_PASSWORD` login has been removed. `APP_API_KEY` remains separate machine-to-machine authentication for scripts and integrations; a LinkedIn browser session also authorizes UI API calls. Health and metrics endpoints and the SPA shell remain public.

Request the **Sign in with LinkedIn using OpenID Connect** product in LinkedIn's Developer Portal and register this exact production callback:

```text
https://research.leadscaptain.com/api/v1/auth/callback
```

Set these values at deploy time (never commit real credentials):

```env
LINKEDIN_CLIENT_ID=your-linkedin-app-client-id
LINKEDIN_CLIENT_SECRET=your-linkedin-app-client-secret
LINKEDIN_REDIRECT_URL=https://research.leadscaptain.com/api/v1/auth/callback
LINKEDIN_ISSUER_URL=https://www.linkedin.com/oauth
LINKEDIN_SCOPES=openid profile email
AUTH_SESSION_SECRET=replace-with-output-of-openssl-rand-base64-48
```

The server reports a clear configuration error until all required variables are present. `LINKEDIN_REDIRECT_URL` must exactly match the redirect URL registered in LinkedIn. The login uses authorization code flow, OIDC discovery and ID-token verification, state, nonce and PKCE. A successful callback upserts the local user by LinkedIn's stable `sub`, stores the granted access/refresh tokens and expiry for permitted official API calls, then creates a signed seven-day HttpOnly/SameSite session. Logout clears the local session. Cancelled consent, invalid/expired state, missing codes and invalid ID tokens fail without creating a user or session.

Endpoints:

- `GET /api/v1/auth/login` - starts the LinkedIn redirect.
- `GET /api/v1/auth/callback` - validates LinkedIn's response, upserts the user and redirects to `/`. This URL is called by LinkedIn, not by hand.
- `GET /api/v1/auth/me` - returns the current local user's `name`, optional `email` and optional `picture`.
- `POST /api/v1/auth/logout` - clears the local session.

```bash
# Browser entry point (returns a LinkedIn redirect)
curl -I https://research.leadscaptain.com/api/v1/auth/login

# After browser authentication, inspect/clear the local session cookie
curl -b cookies.txt https://research.leadscaptain.com/api/v1/auth/me
curl -b cookies.txt -X POST https://research.leadscaptain.com/api/v1/auth/logout

# Non-browser API clients still use APP_API_KEY
curl -H "Authorization: Bearer $APP_API_KEY" \
  https://research.leadscaptain.com/api/v1/stats
```

### Current LinkedIn API limits

Authentication needs the self-service **Sign in with LinkedIn using OpenID Connect** product and the `openid profile email` scopes. `email` and `email_verified` are optional claims even when requested, so the application maps users by `sub`, not email. LinkedIn says this sign-in product authenticates an account but does not verify a person's real-world identity.

The sign-in scopes do **not** grant a general people search. LinkedIn's Connections API is restricted to developers approved by LinkedIn and only returns the consenting member's first-degree connections. It cannot browse another member's connections and does not expose second-degree connections. The Profile API is also restricted for access beyond the authenticated member; looking up another member requires a LinkedIn Person ID obtained through an approved limited-access API and remains subject to privacy settings. Sales-oriented profile matching requires approval for the Sales Navigator Application Platform and its `r_sales_nav_profiles` permission. Do not add scraping as a fallback. Request the relevant product/partner approval in the Developer Portal, then add only the scopes LinkedIn actually grants to `LINKEDIN_SCOPES`; users must re-authenticate after scope changes. Programmatic refresh tokens are available only to a limited set of approved partners.

Official references:

- https://learn.microsoft.com/en-us/linkedin/consumer/integrations/self-serve/sign-in-with-linkedin-v2
- https://learn.microsoft.com/en-us/linkedin/shared/authentication/authorization-code-flow
- https://learn.microsoft.com/en-us/linkedin/shared/authentication/getting-access
- https://learn.microsoft.com/en-us/linkedin/shared/integrations/people/connections-api
- https://learn.microsoft.com/en-us/linkedin/shared/integrations/people/profile-api

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

## Objective leads and debugging API

The Objectives screen links each objective to `/objectives/:id/leads`. The page is isolated by the objective's database ID and combines its progress, paginated lead table, CSV export and objective-filtered SSE debugger.

```bash
# Objective metadata and scoped progress
curl -b cookies.txt localhost:7001/api/v1/objectives/42

# Search/filter/sort one objective's leads
curl -b cookies.txt 'localhost:7001/api/v1/leads?objective_id=42&search=acme&min_score=70&status=NEW&source=leadscaptain&sort=score&direction=desc&page=1&per_page=25'

# Export only that objective's leads
curl -b cookies.txt -OJ 'localhost:7001/api/v1/leads/export?objective_id=42'

# Stream only events carrying objective_id=42 in their payload
curl -N -b cookies.txt 'localhost:7001/api/v1/events?objective_id=42'
```

`GET /api/v1/leads` returns `data`, `page`, `per_page`, `total`, and `last_page`. Supported filters are `objective_id`, `search`, `country`, `status`, `source`, and `min_score`; sort keys are `created_at`, `updated_at`, `name`, `company`, `score`, `status`, and `source`. SSE filtering is performed on the server, so events from other objectives never enter the selected objective's browser log. The client reconnects with bounded exponential backoff and closes the stream on navigation.

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
  `https://api.leadscaptain.com`.
- Ingestion order per lead: check the company website is still up, verify the
  email address, then score with both verdicts. The pipeline uses `https://validation.laravelmail.com/api/v1/verify-email` first (override the base with `EMAIL_VALIDATION_URL`) and falls back to the
  built-in verifier on API errors (syntax, disposable/role, MX, plus
  an SMTP recipient probe with catch-all detection) - nothing is left
  silently `unchecked`. Set `EMAIL_SMTP_PROBE=0` to skip SMTP probing (some
  hosts block outbound port 25; the prober detects that and disables itself
  for the run). `EMAIL_SMTP_HELO` / `EMAIL_SMTP_FROM` tune the probe
  handshake. Verification verdicts rescore leads through the standard score
  breakdown (recorded in `lead_score_history`), never hard-coded values.
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

## Companies section

Search for companies by name, domain, industry keywords or location and open a
merged profile: firmographics from official registries, description and tech
stack from the company website, and social links (LinkedIn, X, Facebook,
Crunchbase). Results can be exported to CSV or converted into a lead with one
click (deduped on company domain).

Data sources (all free by default, nothing is spent):

- **Autocomplete / suggestions** - Clearbit's keyless autocomplete endpoint.
- **Firmographics** - official European registries via `companyreg` (GB
  Companies House needs the free key, FR gouv.fr is keyless; a domain's ccTLD
  routes the lookup when no country is picked).
- **Description, tech stack, socials** - the company's own homepage
  (`webcheck`), no API key.
- **Employee count / revenue** - not available from free sources; set
  `CLEARBIT_API_KEY` to enrich profiles from the Clearbit Company API. The UI
  says so honestly when the key is missing.

Endpoints: `GET /api/v1/companies/search`, `GET /api/v1/companies/suggest`,
`GET /api/v1/companies/profile`, `GET /api/v1/companies` (cache),
`GET /api/v1/companies/export`, `POST /api/v1/companies/save-to-lead`.
## Websites section

Analyze any URL for technical and SEO health: meta title/description, H1 tags,
page-load time, contact scraping (emails, phones and social links from the
homepage and contact pages), keyword density and a 0-100 health score with a
per-check breakdown. Reports are stored (`website_analyses` table) so past
analyses stay browsable.

Everything is fetched from the site itself - no paid API. Traffic estimates
(monthly visitors, sources) have no free data source; the report says so
honestly instead of inventing numbers.

Endpoints: `POST /api/v1/websites/analyze`, `GET /api/v1/websites`,
`GET /api/v1/websites/{id}`.
## Tools suite

Utility tools for manual research and validation, all free to run:

- **Email verification** - syntax check, MX record lookup, provider
  identification (Gmail, Outlook/Microsoft 365, Yahoo, Proton and corporate
  gateways), SMTP recipient probing with catch-all detection (when
  `EMAIL_SMTP_PROBE` is on), disposable-domain and role-address flags ->
  Valid / Invalid / Risky / Unknown. The lead pipeline uses `validation.laravelmail.com` first and falls back to
  these local checks if it is unavailable. `EMAIL_VALIDATION_URL` overrides the
  service base URL.
- **Domain age checker** - creation date, expiry and registrar from RDAP
  (rdap.org bootstrap, free and keyless, the registry-run successor to WHOIS).
- **LinkedIn URL formatter** - normalizes any LinkedIn profile/company link
  (tracking parameters, regional hosts, bare handles) to the canonical
  `https://www.linkedin.com/in/<slug>` form.
- **Bulk processing** - paste up to 25 emails or URLs; queued and processed in
  the background with live progress (`tool_jobs` table).

Endpoints: `POST /api/v1/tools/verify-email`, `GET /api/v1/tools/domain-age`,
`POST /api/v1/tools/linkedin-format`, `POST /api/v1/tools/bulk`,
`GET /api/v1/tools/bulk/{id}`.
