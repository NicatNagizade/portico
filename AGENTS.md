# Portico

Sync API + admin UI. Sources (MySQL / Postgres today) → destinations (Typesense today). Structure is ready for more pairs (e.g. MySQL → MySQL, MySQL → MongoDB) without rewriting the orchestrator.

## Layout

| Path | Role |
|------|------|
| `backend/` | Go API — Gin + GORM |
| `frontend/` | React admin UI — Vite + Tailwind |
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
make example-migrate && make example-seed
```

No Docker in the default workflow — run locally with `go run` / Vite. App DB is created on migrate if missing (`DB_*` vars, never `DATABASE_URL`).

## Working style (non-negotiable)

These come from how this project is built day to day. Prefer them over “clever” defaults.

- **Keep it simple.** Smallest change that works. If a solution needs timeouts, polling intervals, extra tables, custom ordering tags, or parallel code paths — stop and simplify.
- **Readable over abstract.** Prefer clear neighboring patterns; avoid new frameworks/helpers unless necessary.
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

- **Connections** — `mysql` | `postgres` | `typesense` (| `mongodb` reserved). Config JSON sealed at rest.
- **Sync jobs** — `source_connection_id` → `source_table` → `destination_connection_id` → `destination_table`, plus `chunk_size`, `workers`, opaque `config`.
- **Fields** — optional overrides (rename / type / exclude). Omit all → pass through every source column. `active=false` → exclude from import. Coerce values to the declared destination type before write (e.g. object → string when type is string). Nullable `sync_job_relation_id` scopes a field to a relation’s related rows; omit for root/source fields.
- **Relations** — `belongs_to_many` / `has_many` / `has_one` / `belongs_to` (+ optional `parent_id` pointing at another `sync_job_relations.id` for nesting). `belongs_to_many` stores `pivot_table` inside relation `config` JSON. Always select all related columns. Empty FK/key fields fall back to the related field name. Emit arrays/nested objects in the destination doc — not flattened joins. `active=false` skips the relation.
- **Sync logs** — status, `rows_total` (source count at start), `rows_synced` (updated after each chunk), `duration_ms` (updated with progress). Progress UI = `rows_synced / rows_total` — nothing fancier.
- **Sync behavior** — destination is prepared/cleared then bulk-written in chunks from the job’s `chunk_size`.

## When unsure

1. Prefer the simple path used in neighboring files.
2. Prefer GORM + existing connector interfaces.
3. Ask before adding infra (Docker, new services, polling, raw SQL, extra migrations for cosmetics).
