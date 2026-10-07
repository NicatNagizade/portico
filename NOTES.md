# Portico notes

Short gotchas for agents. Prefer `AGENTS.md` / `SKILLS.md` for full rules. Update this file when you learn something durable and brief.

## Stack

- Backend: Go (Gin + GORM). Frontend: React + Vite + Tailwind.
- No Docker by default — `make api` / `make frontend` locally.
- App DB env is `DB_*` only — never `DATABASE_URL`.

## Sync

- Destination connectors own opaque job `config` JSON; defaults when keys are omitted.
- Progress is dumb: `rows_synced / rows_total` (+ `duration_ms`). No timers/polling unless asked.
- New connector type = package under `connectors/<name>/` + registry. Do not touch the orchestrator.
- `config.primary_key` maps source → destination key (default `id`). See `SKILLS.md`.

## Style

- Smallest readable change. No speculative timeouts, polling, or extra tables.
- Tests only in `backend/tests` and `frontend/tests`.
- Secrets sealed with `APP_KEY` (env only).
