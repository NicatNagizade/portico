# Portico

Sync API + admin UI. Sources (MySQL / Postgres) → destinations (Typesense / MongoDB / MySQL / Postgres). Structure is ready for more pairs without rewriting the orchestrator.

## Layout

| Path | Role |
|------|------|
| `backend/` | Go API — Gin + GORM |
| `backend/internal/handlers/` | HTTP handlers split by concern (`handlers.go`, `connections.go`, `sync_jobs.go`, `sync_logs.go`) |
| `backend/internal/connectors/` | Source/destination connectors; shared SQL in `sqlutil/` |
| `frontend/` | React admin UI — Vite + Tailwind |
| `frontend/src/api/` | Thin API clients (`client.js` + resource modules) |
| `exampleData/` | Seed Postgres DB for nested-relation demos |
| `SKILLS.md` | Task playbooks (tests, connectors, sync, UI, …) |

## Commands

```bash
make setup                              # deps + .env files
make api                                # backend :8080
make frontend                           # Vite :5173 (proxies /api)
make test                               # backend tests (needs CGO)
make swagger                            # regenerate backend/docs/swagger.json only
make lint                               # frontend oxlint
make migrate-refresh                    # DESTRUCTIVE: drop all app tables + AutoMigrate
make import-connections                 # upsert from backend/connections.json (see .example)
make example-migrate && make example-seed
```

No Docker in the default workflow — run locally with `go run` / Vite. App DB is created on migrate if missing (`DB_*` vars, never `DATABASE_URL`). Copy `backend/connections.json.example` → `backend/connections.json` (gitignored) to bootstrap connector credentials and optional sync jobs (fields/rules/relations) into the DB; optional `CONNECTIONS_FILE` overrides the path.

## Working style (non-negotiable)

These come from how this project is built day to day. Prefer them over “clever” defaults.

- **Keep it simple.** Smallest change that works. If a solution needs timeouts, polling intervals, extra tables, custom ordering tags, or parallel code paths — stop and simplify.
- **Readable over abstract.** Prefer clear neighboring patterns; avoid new frameworks/helpers unless necessary.
- **DRY.** Do not copy the same logic across connectors, handlers, or pages — extract a small shared helper (or use a package) when repetition appears. Prefer one clear place over “almost the same” copies.
- **Clear folders.** Keep concerns in the right place (`connectors/<name>/`, `services/sync/`, frontend `api/` + pages). Split oversized files when a second concern grows; do not invent deep package trees for cosmetics.
- **GORM first.** Use the ORM for app DB reads/writes. Do not reach for raw SQL unless GORM cannot express it cleanly.
- **No speculative features.** Do not add auto-refresh, request timeouts, or background intervals unless explicitly asked.
- **Minimal API chatter.** Frontend schema/autocomplete calls only when necessary (cache / load once per connection).
- **Match field/model order.** Trust struct/`gorm` column order in models; do not invent separate “order” metadata.

## Conventions

### Tests

- All tests live in `backend/tests` or `frontend/tests` — never next to source.
- See `SKILLS.md` → Writing tests.

### Connectors

- Add sources/destinations under `backend/internal/connectors/<name>/`.
- Register in `register.DefaultRegistry()`.
- Do **not** change `services/sync` orchestrator for a new connector type — implement the `SourceReader` / `DestinationWriter` interfaces.
- See `SKILLS.md` → Extending connectors.

### Secrets & config

- Connection passwords/API keys are sealed with `APP_KEY` (process env only — never store the key in the DB).
- Keep `secretbox` small; do not grow encryption helpers without need.
- Sync job `config` is **opaque JSON** interpreted by the destination connector (defaults when keys are omitted). Field-level destination config is per-field overrides.

### API / DB

- Swagger: update swag comments → `make swagger` → **JSON only** (`backend/docs/swagger.json`). UI at `/api/documentation`.
- Status-like columns: store as tinyint/int in DB; map meanings in Go/JS constants — no DB enum constraints.
- Prefer ON DELETE CASCADE (or equivalent) for parent→child rows (e.g. sync job → sync logs) so deletes work without manual cleanup.
- Env: `DB_DRIVER`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `APP_KEY`, `HTTP_ADDR`.

### Frontend

- Manual refresh (button + loading state) over silent auto-polling, unless the user asks for live updates.
- Prefer simple, readable form code (autocomplete, editors) over heavy abstraction.
- Paginate list endpoints/pages when lists can grow (sync jobs, sync logs).

## Domain model

- **Connections** — `mysql` | `postgres` | `typesense` | `mongodb`. Config JSON sealed at rest.
- **Sync jobs** — `source_connection_id` → `source_table` → `destination_connection_id` → `destination_table`, plus `chunk_size`, `workers`, opaque `config`.
- **Fields** — optional overrides (rename / type / exclude / value maps). Omit all → pass through every source column. `active=false` → exclude from import. Coerce values to the declared destination type before write (e.g. object → string when type is string). Nullable `sync_job_relation_id` scopes a field to a relation’s related rows; omit for root/source fields. Optional `values` (`sync_job_fields_values`) map source cell values to destination values (e.g. `1` → `success`); unmatched values pass through.
- **Rules** — optional source filters (`field` + `operator` + `value`) AND’d before Count/Read. Operators: `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `not_in`, `like`, `is_null`, `is_not_null`. `active=false` skips the rule. Applied at the source SQL layer (not post-fetch).
- **Relations** — `belongs_to_many` / `has_many` / `has_one` / `belongs_to` (+ optional `parent_id` pointing at another `sync_job_relations.id` for nesting). `belongs_to_many` stores `pivot_table` inside relation `config` JSON. Always select all related columns. Empty FK/key fields fall back to the related field name. Emit arrays/nested objects in the destination doc — not flattened joins. `active=false` skips the relation.
- **Sync logs** — status, `rows_total` (source count at start), `rows_synced` (updated after each chunk), `duration_ms` (updated with progress). Progress UI = `rows_synced / rows_total` — nothing fancier.
- **Sync behavior** — destination is prepared/cleared then bulk-written in chunks from the job’s `chunk_size`.

## When unsure

1. Prefer the simple path used in neighboring files.
2. Prefer GORM + existing connector interfaces.
3. Ask before adding infra (Docker, new services, polling, raw SQL, extra migrations for cosmetics).
