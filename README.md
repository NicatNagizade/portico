# Portico

Sync API and admin UI. Define **connections** (MySQL, Postgres, SQLite, Typesense, MongoDB, Redis — each as source or destination), configure **sync jobs** (tables, fields, rules, nested relations), and run chunked syncs with progress logs.

| Path | Role |
|------|------|
| `backend/` | Go API (Gin + GORM) on `:8080` |
| `frontend/` | React admin UI (Vite + Tailwind) on `:5173` |
| `exampleData/` | Seed Postgres/MySQL social graph for sync demos |
| `backend/connections.json` | Optional bootstrap of connector credentials + sync jobs (gitignored) |

## Prerequisites

- Go 1.22+
- Node.js 20+
- Local Postgres (app DB + example dataset)
- Optional for full demos: Typesense (`:8108`), MongoDB (`:27017`), Redis (`:6379`)

## Quick start (from zero)

```bash
# 1) Install deps and create .env files
make setup

# 2) Seed the example social dataset (source DB for demos)
make example-migrate
make example-seed          # long at default volume — shrink USER_COUNT first for a smoke test

# 3) Bootstrap connector credentials + sample sync jobs into the app DB
cp backend/connections.json.example backend/connections.json
make import-connections

# 4) Run API + UI (two terminals)
make api                   # http://localhost:8080
make frontend              # http://localhost:5173 (proxies /api)
```

Swagger UI: [http://localhost:8080/api/documentation](http://localhost:8080/api/documentation)

---

## 1. Example dataset (`exampleData/`)

Large fake social graph used as the **source** in sample sync jobs: `users` → `posts` → `comments` → `reactions`.

### Credentials (defaults in `exampleData/.env.example`)

| Variable | Default | Notes |
|----------|---------|--------|
| `DB_DATABASE` | `postgres` | Or `mysql` |
| `DB_HOST` | `localhost` | |
| `DB_PORT` | `5432` | `3306` for MySQL |
| `DB_USER` | `postgres` | `root` typical for MySQL |
| `DB_PASSWORD` | `postgres` | |
| `DB_NAME` | `portico_example` | Created by migrate if missing |
| `DB_SSLMODE` | `disable` | Postgres only |

`make example-migrate` / `make example-seed` copy `.env.example` → `.env` if needed.

### Seed volume

Default is large (`USER_COUNT=1000000`). For a quick local smoke test, edit `exampleData/.env`:

```env
USER_COUNT=100
USER_BATCH_SIZE=50
```

### Commands

```bash
make example-migrate              # create DB + tables
make example-migrate-truncate     # DESTRUCTIVE: drop example tables, recreate
make example-seed                 # truncate + load data
```

Or from `exampleData/`: `go run . migrate` / `go run . seed`. Details: [exampleData/README.md](exampleData/README.md).

---

## 2. Backend (`backend/`)

App metadata (connections, sync jobs, logs) lives in its **own** Postgres DB — separate from `portico_example`.

### Credentials (defaults in `backend/.env.example`)

| Variable | Default | Notes |
|----------|---------|--------|
| `HTTP_ADDR` | `:8080` | Listen address |
| `DB_DRIVER` | `postgres` | App DB |
| `DB_HOST` | `localhost` | |
| `DB_PORT` | `5432` | |
| `DB_USER` | `postgres` | |
| `DB_PASSWORD` | `postgres` | |
| `DB_NAME` | `portico` | Created on startup / migrate if missing |
| `DB_SSLMODE` | `disable` | |
| `APP_KEY` | (see `.env.example`) | Encrypts connection passwords/API keys at rest. Set before creating connections; changing it later cannot decrypt existing rows. |
| `OPENAI_API_KEY` | empty | Optional — Explore → Ask AI |
| `OPENAI_BASE_URL` / `OPENAI_MODEL` | | Optional OpenAI-compatible LLM |

`make setup` copies `.env.example` → `.env` and runs `go mod tidy`. On API start, the app creates `DB_NAME` if missing and auto-migrates.

### Run

```bash
make api
# or: cd backend && go run ./cmd/api
```

Other useful targets: `make migrate-refresh` (DESTRUCTIVE: drop app tables + AutoMigrate), `make swagger`, `make test`.

Full env/API notes: [backend/README.md](backend/README.md).

---

## 3. Bootstrap connections (`backend/connections.json`)

Copy the example and import so the admin UI already has a source DB plus destination connectors and sample jobs:

```bash
cp backend/connections.json.example backend/connections.json
# edit hosts/secrets if your local services differ
make import-connections
```

`connections.json` is gitignored. Override path with `CONNECTIONS_FILE`. Import **upserts** connections by `(name, type)` and sync jobs by `name`.

### What the example defines

**Connections** (match typical local defaults):

| Name | Type | Credentials |
|------|------|-------------|
| `local-test-db` | postgres | `user` / `password` @ `localhost:5432` / DB `portico_example` |
| `local-typesense` | typesense | `http://localhost:8108`, API key `xyz` |
| `local-mongo` | mongodb | `user` / `password` @ `127.0.0.1:27017` / DB `portico_test`, `auth_source=admin` |
| `local-redis` | redis | `localhost:6379`, password `eYVX7EwVmmxKPCDmwMtyKVge8oLd2t81` |

**Sync jobs** (all read `posts` from `local-test-db` with nested `user` / `comments` / `reactions`):

- `posts to typesense` → Typesense collection `posts` (`enable_nested_fields: true`)
- `posts to mongo db` → MongoDB collection `posts`
- `posts to redis` → Redis keys `{table}:{id}`

Align these secrets with how you actually run Typesense / Mongo / Redis locally (or remove unused entries before import).

---

## 4. Frontend (`frontend/`)

React admin for connections, sync jobs, and sync logs.

### Setup

```bash
make setup-frontend
# or: cd frontend && cp .env.example .env && npm install
```

| Variable | Default | Notes |
|----------|---------|--------|
| `VITE_API_BASE` | `/api` | Proxied to the backend in dev |

### Run

```bash
make frontend
# or: cd frontend && npm run dev
```

Open [http://localhost:5173](http://localhost:5173). Keep the API running on `:8080`.

Production build: `make build` / `make preview`. Details: [frontend/README.md](frontend/README.md).

---

## Make targets

```bash
make setup                    # backend + frontend deps / .env
make api                      # Go API :8080
make frontend                 # Vite :5173
make import-connections       # upsert from backend/connections.json
make example-migrate && make example-seed
make migrate-refresh          # DESTRUCTIVE: reset app DB schema
make test                     # backend tests (needs CGO)
make test-frontend            # Playwright
make swagger                  # regenerate backend/docs/swagger.json
make lint                     # frontend oxlint
```

No Docker in the default workflow — run services locally (`go run` / Vite) and point env / `connections.json` at them.
