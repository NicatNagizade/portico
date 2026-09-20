.PHONY: help setup setup-backend setup-frontend \
	api frontend test build tidy swagger lint preview \
	example-migrate example-seed

BACKEND := backend
FRONTEND := frontend
EXAMPLE := exampleData

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

setup: setup-backend setup-frontend ## Install deps and create .env files

setup-backend: ## Copy backend .env and tidy Go modules
	@test -f $(BACKEND)/.env || cp $(BACKEND)/.env.example $(BACKEND)/.env
	cd $(BACKEND) && go mod tidy

setup-frontend: ## Copy frontend .env and install npm packages
	@test -f $(FRONTEND)/.env || cp $(FRONTEND)/.env.example $(FRONTEND)/.env
	cd $(FRONTEND) && npm install

api: ## Run the Go API (:8080)
	cd $(BACKEND) && go run ./cmd/api

frontend: ## Run the Vite dev server (:5173)
	cd $(FRONTEND) && npm run dev

test: ## Run backend tests (requires CGO)
	cd $(BACKEND) && CGO_ENABLED=1 go test ./tests/...

build: ## Build frontend for production
	cd $(FRONTEND) && npm run build

preview: ## Preview production frontend build
	cd $(FRONTEND) && npm run preview

tidy: ## Tidy Go modules
	cd $(BACKEND) && go mod tidy

swagger: ## Regenerate OpenAPI JSON
	cd $(BACKEND) && go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go -o docs -ot json

lint: ## Lint frontend
	cd $(FRONTEND) && npm run lint

example-migrate: ## Create example Postgres DB + tables
	@test -f $(EXAMPLE)/.env || cp $(EXAMPLE)/.env.example $(EXAMPLE)/.env
	cd $(EXAMPLE) && go run . migrate

example-seed: ## Seed fake users/posts/comments/reactions
	@test -f $(EXAMPLE)/.env || cp $(EXAMPLE)/.env.example $(EXAMPLE)/.env
	cd $(EXAMPLE) && go run . seed
