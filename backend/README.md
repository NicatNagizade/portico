# Portico Sync API

Go API for managing connections and syncing tables (MySQL/Postgres → Typesense for now), built with Gin + GORM.

## Prerequisites

- Go 1.22+
- Local Postgres (no Docker Compose)

## Setup

```bash
cd backend
cp .env.example .env
go mod tidy
```

On startup the API creates the Postgres database from `DB_NAME` if it does not exist, then auto-migrates.

## Run

```bash
go run ./cmd/api
```

API listens on `HTTP_ADDR` (default `:8080`).

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
| `APP_KEY` | Application secret used to encrypt connection passwords and API keys at rest. It is not stored in the database. Empty uses a development key. Changing it makes existing secrets unreadable. | |

## Swagger

Open [http://localhost:8080/api/documentation](http://localhost:8080/api/documentation).

OpenAPI spec is JSON only: [`docs/swagger.json`](docs/swagger.json).

Regenerate after changing annotations:

```bash
go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go -o docs -ot json
```

## Tests

Requires CGO (SQLite driver):

```bash
CGO_ENABLED=1 go test ./tests/...
```

## API overview

- `GET/POST /connections`, `GET/PUT/DELETE /connections/:id`
- `GET/POST /sync-jobs`, `GET/PUT/DELETE /sync-jobs/:id`
- `POST /sync-jobs/:id/run` — full reload sync (drop destination collection if present, recreate, bulk insert)
- `GET /sync-logs`, `GET /sync-logs/:id`
- `GET /health`

## Example: create connections

`POST /connections` with `Content-Type: application/json`.

### MySQL (source)

```bash
curl -X POST http://localhost:8080/connections \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "mysql-app",
    "type": "mysql",
    "config": {
      "host": "localhost",
      "port": 3306,
      "user": "root",
      "password": "secret",
      "database": "app"
    }
  }'
```

### Typesense (destination)

```bash
curl -X POST http://localhost:8080/connections \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "typesense-local",
    "type": "typesense",
    "config": {
      "host": "localhost",
      "port": 8108,
      "api_key": "xyz",
      "protocol": "http"
    }
  }'
```

## Extending connectors

Sources and destinations live under `internal/connectors/`. Implement `SourceReader` or `DestinationWriter`, then register in `DefaultRegistry()`. Future destinations (MySQL, MongoDB) can be added the same way without changing the sync orchestrator.

## Example: sync with related tables

Many-to-many relations (e.g. applicants ↔ tags via `applicant_tags`) use `belongs_to_many` with a pivot table. One-to-many FK children use `has_many`. Nest deeper levels with `parent_relation` (name of another relation on the same job). Related rows are loaded with separate queries (no JOINs) and attached as Typesense `object[]` fields.

```bash
curl -X POST http://localhost:8080/sync-jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "applicants-to-typesense",
    "source_connection_id": 1,
    "destination_connection_id": 2,
    "source_table": "applicants",
    "destination_table": "applicants",
    "config": {
      "default_sorting_field": "id",
      "enable_nested_fields": true
    },
    "relations": [{
      "name": "tags",
      "type": "belongs_to_many",
      "table": "tags",
      "pivot_table": "applicant_tags",
      "active": true
    }],
    "fields": [{
      "source_name": "full_name",
      "destination_name": "name",
      "destination_type": "string",
      "active": true
    }, {
      "source_name": "internal_notes",
      "destination_name": "notes",
      "destination_type": "string",
      "active": false
    }]
  }'
```

### ExampleData: nested users → posts → comments → reactions

Connect the seeded Postgres DB (`exampleData`, default `portico_test` / `portico_example`) as a Postgres source, then sync `users` with nested `has_many` relations. Keep Typesense `enable_nested_fields: true`.

```bash
# 1) Postgres source → exampleData DB
curl -X POST http://localhost:8080/connections \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "exampledata-postgres",
    "type": "postgres",
    "config": {
      "host": "localhost",
      "port": 5432,
      "user": "user",
      "password": "password",
      "database": "portico_test",
      "sslmode": "disable"
    }
  }'

# 2) Sync job (replace connection IDs)
curl -X POST http://localhost:8080/sync-jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "users-nested-social",
    "source_connection_id": 1,
    "destination_connection_id": 2,
    "source_table": "users",
    "destination_table": "users",
    "chunk_size": 100,
    "config": {
      "default_sorting_field": "id",
      "enable_nested_fields": true
    },
    "relations": [
      {
        "name": "posts",
        "type": "has_many",
        "table": "posts",
        "foreign_key": "user_id"
      },
      {
        "name": "comments",
        "type": "has_many",
        "table": "comments",
        "foreign_key": "post_id",
        "parent_relation": "posts"
      },
      {
        "name": "reactions",
        "type": "has_many",
        "table": "reactions",
        "foreign_key": "comment_id",
        "parent_relation": "comments"
      }
    ]
  }'
```

Document shape after sync:

```json
{
  "id": "1",
  "username": "user_1",
  "posts": [
    {
      "id": 10,
      "title": "...",
      "comments": [
        {
          "id": 100,
          "body": "...",
          "reactions": [{"id": 1, "type": "like"}]
        }
      ]
    }
  ]
}
```

Optional per-field overrides live in `fields`. With no rows, columns pass through unchanged. An active row can rename via `destination_name` and override the destination schema type via `destination_type` (`string`, `int64`, `float64`, `bool`, `object`, `object_array`). `active: false` excludes the field from the destination schema and imported documents. Opaque `destination_config` is stored for future destination-specific options.

Relations support the same `active` flag: `active: false` skips enrichment and omits that relation from the destination schema (root-level only; nested relations are not listed in the destination schema and rely on `enable_nested_fields`). Omit `active` (or set `true`) to keep the relation enabled.

Optional sync-job `config` is opaque JSON interpreted by the destination connector. Omit it (or any key) to keep that connector's defaults. For Typesense: `default_sorting_field`, `enable_nested_fields` (default `true`), `symbols_to_index`, `token_separators`. Other connectors can define their own keys.

Omitting `foreign_key` / `related_key` defaults them to `{singular(parent_table)}_id` and `{singular(related_table)}_id` (e.g. `applicant_id`, `tag_id`). For nested `has_many`, the parent table is the parent relation’s `table`. All columns from the related table are included. After a `belongs_to_many` sync, each applicant document looks like:

```json
{
  "id": "42",
  "name": "Ada",
  "tags": [
    {"id": 1, "name": "vip"},
    {"id": 2, "name": "referral"}
  ]
}
```

