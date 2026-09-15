# Autonomous Lead Research Agent
Go + React lead research platform. An agent engine plans iterations with an
LLM, calls tools (lead search, email verification, statistics), scores and
stores leads, and drafts outreach - all over a chi REST API with a React
frontend and a SQLite store.

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
- `CORS_ALLOWED_ORIGINS` is honored when set (comma-separated); defaults to `*`.

## Tests
go test ./...

## Docs
- docs/architecture.md
- docs/patents/lead-research-patent-landscape.md
