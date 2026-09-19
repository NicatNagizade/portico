# Portico Admin Frontend

React admin UI for the Portico Sync API (connections, sync jobs, sync logs).

## Prerequisites

- Node.js 20+
- Backend API running on `http://localhost:8080` (see `../backend/README.md`)

## Setup

```bash
cd frontend
cp .env.example .env
npm install
```

## Run

```bash
npm run dev
```

Opens on the Vite default port (usually `http://localhost:5173`). API calls go to `/api/*` and are proxied to the backend.

## Build

```bash
npm run build
npm run preview
```

## Environment

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_BASE` | API base path (proxied in dev) | `/api` |
