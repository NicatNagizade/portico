# Portico Sync API

Node API for connections and sync jobs (MySQL / Postgres / SQLite / Typesense / MongoDB / Redis as source or destination). Express + Drizzle.

## Setup

```bash
cp .env.example .env
npm install
npm run api
```

Listens on `HTTP_ADDR` (default `:8080`). On startup, creates `DB_NAME` if missing and applies migrations in `src/db/migrations/`.

From the repo root: `make api`, `make migrate-refresh`, `make import-connections`, `make run-sync ID=1`, `make test`.

### Run a sync job

```bash
make run-sync ID=1
# or: npm run run-sync -- 1
```

Runs the job in this process (same path as `POST /sync-jobs/{id}/run`) and prints the finished sync log. Ctrl+C stops it. Exit code is non-zero when the sync fails.

### Import connections (+ sync jobs)

```bash
cp connections.json.example connections.json
# edit secrets / jobs, then:
make import-connections   # from repo root
```

Upserts connections by `(name, type)` and sync jobs by `name`. Jobs reference connections by name; nested `relations` (tree via `relations[]`) / `fields` / `rules` / `values` match the sync-job API. Override path with `CONNECTIONS_FILE`.

## Environment

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_ADDR` | Listen address | `:8080` |
| `DB_DRIVER` | App DB driver (`postgres` or `sqlite`) | `postgres` |
| `DB_HOST` | App DB host | `localhost` |
| `DB_PORT` | App DB port | `5432` |
| `DB_USER` | App DB user | `postgres` |
| `DB_PASSWORD` | App DB password | `postgres` |
| `DB_NAME` | App DB name | `portico` |
| `DB_SSLMODE` | SSL mode | `disable` |
| `APP_KEY` | Encrypts connection secrets at rest (not stored in DB). Empty = dev key. | |
| `OPENAI_API_KEY` | Optional. Enables Explore → Ask AI (OpenAI-compatible). | |
| `OPENAI_BASE_URL` | Optional chat API base (e.g. OpenRouter). | `https://api.openai.com/v1` |
| `OPENAI_MODEL` | Optional model id. | `gpt-4o-mini` |
| `CONNECTIONS_FILE` | Path for `import-connections` | `connections.json` |

## Tests

```bash
npm test
# or: make test
```

## Schema

The app database (connections, sync jobs, logs) goes through Drizzle. `openDb` applies `src/db/migrations/`. If those tables already exist and drizzle has no migration record, the current migrations are recorded as applied and are not run again. `npm run db:generate` writes a new Postgres and SQLite migration after a schema change. Connector queries stay raw SQL under `src/connectors/sql/`; those databases are the user's, and their tables are not known at compile time.

`make migrate-refresh` drops all app tables and recreates the schema.

## Layout

| Path | Role |
|---|---|
| `src/index.ts`, `src/app.ts` | Process entry and Express app wiring |
| `src/cli/` | `migrate`, `import-connections`, `run-sync` entrypoints |
| `src/http/` | Routes per resource, request helpers, JSON presenters |
| `src/domain/` | Shared types (connection, sync job, …) and HTTP errors |
| `src/db/` | Drizzle client, typed schema, generated migrations (app database only) |
| `src/repositories/` | App DB reads/writes per table, row → domain mappers |
| `src/services/syncJobs/` | Create/update a sync job with nested relations, fields, rules |
| `src/services/sync/` | Orchestrator (logs, cancel), runner (chunks + workers), relation loading |
| `src/services/explore/` | Explore preview + CSV export for source / destination |
| `src/connectors/<type>/` | One folder per connector: `source.ts`, `destination.ts`, helpers |
| `src/connectors/shared/` | In-memory filters, paging, schema inference used by document stores |
| `src/utils/` | Generic helpers (values, collections, CSV, worker pool, secretbox) |

Add a connector by creating `src/connectors/<type>/` with a `SourceReader` and `DestinationWriter`, then registering both in `connectors/registry.ts`.

## API

- `GET/POST /connections`, `GET/PUT/DELETE /connections/:id`
- `GET/POST /sync-jobs`, `GET/PUT/DELETE /sync-jobs/:id`
- `POST /sync-jobs/:id/run`
- `POST /sync-jobs/:id/explore`, `/explore/suggest`, `/explore/export`
- `GET /sync-logs`, `GET /sync-logs/:id`
- `GET /health`

Domain details (fields, relations, rules, connectors): see repo [AGENTS.md](../AGENTS.md) and [SKILLS.md](../SKILLS.md).
