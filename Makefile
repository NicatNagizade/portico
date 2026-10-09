.PHONY: help setup setup-backend setup-frontend \
	api frontend test test-frontend build lint preview \
	migrate-refresh import-connections run-sync \
	example-migrate example-migrate-truncate example-seed

BACKEND := backend
FRONTEND := frontend
EXAMPLE := exampleData

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-28s %s\n", $$1, $$2}'

setup: setup-backend setup-frontend ## Install deps and create .env files

setup-backend: ## Copy backend .env and install npm packages
	@test -f $(BACKEND)/.env || cp $(BACKEND)/.env.example $(BACKEND)/.env
	cd $(BACKEND) && npm install

setup-frontend: ## Copy frontend .env and install npm packages
	@test -f $(FRONTEND)/.env || cp $(FRONTEND)/.env.example $(FRONTEND)/.env
	cd $(FRONTEND) && npm install

api: ## Run the API (:8080)
	cd $(BACKEND) && npm run api

frontend: ## Run the Vite dev server (:5173)
	cd $(FRONTEND) && npm run dev

test: ## Run backend tests
	cd $(BACKEND) && npm test

test-frontend: ## Run Playwright frontend tests
	cd $(FRONTEND) && npm test

build: ## Build frontend for production
	cd $(FRONTEND) && npm run build

preview: ## Preview production frontend build
	cd $(FRONTEND) && npm run preview

lint: ## Lint frontend
	cd $(FRONTEND) && npm run lint

migrate-refresh: ## DESTRUCTIVE: drop all app tables and re-run migrations
	cd $(BACKEND) && npm run migrate:refresh

import-connections: ## Upsert connections + sync jobs from backend/connections.json
	cd $(BACKEND) && npm run import-connections

run-sync: ## Run a sync job by ID (make run-sync ID=1)
	@test -n "$(ID)" || (echo "usage: make run-sync ID=<sync-job-id>" && exit 2)
	cd $(BACKEND) && npm run run-sync -- $(ID)

example-migrate: ## Create example DB + tables (Postgres/MySQL via DB_DATABASE)
	@test -f $(EXAMPLE)/.env || cp $(EXAMPLE)/.env.example $(EXAMPLE)/.env
	cd $(EXAMPLE) && go run . migrate

example-migrate-truncate: ## DESTRUCTIVE: drop example tables, then recreate schema
	@test -f $(EXAMPLE)/.env || cp $(EXAMPLE)/.env.example $(EXAMPLE)/.env
	cd $(EXAMPLE) && go run . migrate --truncate

example-seed: ## Seed fake users/posts/comments/reactions
	@test -f $(EXAMPLE)/.env || cp $(EXAMPLE)/.env.example $(EXAMPLE)/.env
	cd $(EXAMPLE) && go run . seed
