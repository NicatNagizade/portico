# Portico

Sync API and admin UI for managing connections and syncing tables
(MySQL/Postgres → Typesense).

## Structure

| Path | Description |
|------|-------------|
| `backend/` | Go API (Gin + GORM) |
| `frontend/` | React admin UI (Vite) |

## Prerequisites

- Go 1.22+
- Node.js 20+
- Local Postgres

## Backend

```bash
cd backend
cp .env.example .env
go mod tidy
go run ./cmd/api
```

API listens on `HTTP_ADDR` (default `:8080`).
Swagger: http://localhost:8080/api/documentation

See [backend/README.md](backend/README.md) for env vars, API overview, and tests.

## Frontend

```bash
cd frontend
cp .env.example .env
npm install
npm run dev
```

Dev server (usually http://localhost:5173) proxies `/api` to the backend.

See [frontend/README.md](frontend/README.md) for details.

## Development

Run both processes:

1. Backend on `:8080`
2. Frontend with `npm run dev`
