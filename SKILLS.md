# Portico skills

Task playbooks for common work on this repo. Read `AGENTS.md` for project rules; use this file when doing a concrete change.

**Default rule for every skill:** keep it simple. Smallest readable change. No speculative timeouts, polling, extra tables, or raw SQL when GORM works.

---

## Writing tests

**When:** adding, moving, or updating tests.

- Backend: `backend/tests/` only (`make test` → `go test ./tests/...`).
- Frontend: `frontend/tests/` only.
- Never put `*_test.go` / test files next to source packages.
- Prefer extending existing API tests in `backend/tests/api_test.go` over new frameworks.
- After API shape changes, cover the happy path and one failure case if cheap.

---

## Extending connectors

**When:** new source/destination, or changing ListTables / Schema / Read / Write behavior.

1. Implement under `backend/internal/connectors/<name>/`.
2. Satisfy `SourceReader` and/or `DestinationWriter` in `connectors/connectors.go`.
3. Register in `register.DefaultRegistry()`.
4. Do **not** edit `services/sync` orchestrator for a new type — the registry is the extension point.
5. Destination `Prepare`/`WriteBatch`: treat sync job `config` as opaque JSON; apply defaults when keys are missing.
6. Coerce field values to the declared destination type before write (e.g. stringify when type is string).
7. Update swagger comments + `make swagger` if HTTP surface changes; add/adjust tests under `backend/tests`.

Future pairs (MySQL→MySQL, MySQL→MongoDB) should land the same way — new connector package + registry entry.

---

## Sync jobs, fields, relations

**When:** mapping, overrides, nested imports, source filters.

- No field rows → pass through all source columns.
- `active=false` on a field or relation → skip it on import.
- Rules filter source rows before Count/Read (AND). Example: `field=client_id`, `operator=eq`, `value=123`.
- Operators: `eq` / `neq` / `gt` / `gte` / `lt` / `lte` / `in` / `not_in` / `like` / `is_null` / `is_not_null`.
- `in` / `not_in` values are comma-separated. `is_null` / `is_not_null` ignore value.
- `active=false` on a rule → skip that predicate.
- Relations: `belongs_to_many` / `has_many` / `has_one` / `belongs_to`; optional `parent_id` (FK to another relation row) for nesting.
- M2M pivot table lives in relation `config.pivot_table`.
- Field rows may set `sync_job_relation_id` to override columns on related docs; omit for root fields.
- Always select all related columns (no per-relation column pickers).
- Empty foreign/related keys → fall back to the related field name.
- Emit related rows as arrays / nested objects in the destination document — not SQL joins that flatten many-to-many.
- Job `config` = destination-global; field destination config = per-field (facet, sort, etc. for Typesense).

---

## Sync run progress & logs

**When:** changing run lifecycle, progress UI, or log fields.

Keep progress dumb:

1. At sync start: set `rows_total` from source `Count`.
2. After each chunk: update `rows_synced` and `duration_ms` (from the log row).
3. UI progress = `rows_synced / rows_total` (and show `duration_ms` from the API).
4. Do not add ordering hacks, timers, request timeouts, or auto-poll intervals unless explicitly asked — use a refresh button + loading state.
5. Deleting a sync job must cascade (or equivalent) to its sync logs.

Field order in models: `rows_total` before `rows_synced` — trust the model, don’t invent separate order metadata.

---

## API, swagger, env, DB

**When:** handlers, migrations, env, docs.

- Env: `DB_DRIVER`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `APP_KEY`, `HTTP_ADDR` — never `DATABASE_URL`.
- Create the app database on migrate if it does not exist.
- Refresh schema (destructive): `make migrate-refresh` drops all app tables and re-runs AutoMigrate.
- Status columns: int/tinyint in DB; constants in code — no DB enum constraints.
- Prefer GORM for app DB access.
- Swagger: swag comments → `make swagger` → **only** `backend/docs/swagger.json`. Docs UI: `/api/documentation`.
- List endpoints that can grow (sync jobs, sync logs): support pagination end-to-end (API + UI).

---

## Secrets

**When:** connection config, encryption helpers.

- Seal passwords/API keys with `APP_KEY` (env only — never persist the key in the DB).
- Keep `internal/secretbox` small; redact secrets in API JSON.
- Do not redesign encryption unless asked.

---

## Frontend admin UI

**When:** pages, forms, lists, autocomplete.

- React + Vite + Tailwind; follow existing `components/` / `pages/` patterns.
- Lists: clear table/list layout + pagination where data grows.
- Autocomplete for tables/fields/relations: simple and readable; fetch schema only when necessary (cache per connection).
- No silent auto-refresh. Refresh button with loading on the list is the default.
- Icon actions over crowded text button rows in dense lists.

---

## Example data & nested demos

**When:** demo DB or nested has_many fixtures.

- `exampleData/`: `make example-migrate` then `make example-seed`.
- Use it to exercise nested relations (users → posts → comments → reactions), not production schema.

---

## Bugfix / simplify pass

**When:** something works but feels heavy, or the user says “simplify”.

1. Remove unused paths, timeouts, intervals, and duplicate state.
2. Prefer one obvious data source (e.g. `duration_ms` from sync_logs) over derived client timers.
3. Match neighboring file style; delete code you made obsolete.
4. Re-read `AGENTS.md` “Working style” before shipping.
