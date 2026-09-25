# Portico Sync API

Go API for connections and sync jobs (MySQL / Postgres / SQLite / Typesense / MongoDB / Redis as source or destination), Gin + GORM.

## Setup

```bash
cp .env.example .env
go mod tidy
go run ./cmd/api
```

Listens on `HTTP_ADDR` (default `:8080`). On startup, creates `DB_NAME` if missing and auto-migrates.

From the repo root: `make api`, `make migrate-refresh`, `make import-connections`, `make swagger`, `make test`.

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
| `DB_DRIVER` | App DB driver | `postgres` |
| `DB_HOST` | App DB host | `localhost` |
| `DB_PORT` | App DB port | `5432` |
| `DB_USER` | App DB user | `postgres` |
| `DB_PASSWORD` | App DB password | `postgres` |
| `DB_NAME` | App DB name | `portico` |
| `DB_SSLMODE` | SSL mode | `disable` |
| `APP_KEY` | Encrypts connection secrets at rest (not stored in DB). Empty = dev key. | |
| `CONNECTIONS_FILE` | Path for `import-connections` | `connections.json` |

## Swagger

[http://localhost:8080/api/documentation](http://localhost:8080/api/documentation) · [`docs/swagger.json`](docs/swagger.json)

```bash
make swagger   # from repo root
```

## Tests

```bash
CGO_ENABLED=1 go test ./tests/...
# or: make test
```

## API

- `GET/POST /connections`, `GET/PUT/DELETE /connections/:id`
- `GET/POST /sync-jobs`, `GET/PUT/DELETE /sync-jobs/:id`
- `POST /sync-jobs/:id/run`
- `GET /sync-logs`, `GET /sync-logs/:id`
- `GET /health`

Domain details (fields, relations, rules, connectors): see repo [AGENTS.md](../AGENTS.md) and [SKILLS.md](../SKILLS.md).
