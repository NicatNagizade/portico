# Example social dataset for Portico (Postgres)

Generates a large fake dataset: users → posts → comments → reactions.

## Schema

| Table | Description |
|-------|-------------|
| `users` | Fake accounts |
| `posts` | 10–30 posts per user |
| `comments` | 20–30 comments per post |
| `reactions` | Optional `like` / `dislike` / `smile` / `angry` on comments |

Default volume: **100,000 users** (~2M posts, ~50M comments). Expect a long seed and significant disk use.

## Setup

```bash
cp .env.example .env
# edit DB_* if needed
go mod tidy
```

Requires a running Postgres you can connect to as `DB_USER`.

## Commands

Exactly two commands:

```bash
# 1) Create database (if not exists) + tables + indexes
go run . migrate

# 2) Truncate and load seed data
go run . seed
```

Or from the repo root:

```bash
make example-migrate
make example-seed
```

## Tuning

In `.env` you can shrink the dataset for a quick smoke test:

```env
USER_COUNT=100
USER_BATCH_SIZE=50
```
