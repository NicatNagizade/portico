package main

import (
	"log"

	"github.com/portico/backend/internal/connectors/register"
	"github.com/portico/backend/internal/db"
	"github.com/portico/backend/internal/handlers"
	"github.com/portico/backend/internal/llm"
	"github.com/portico/backend/internal/router"
	"github.com/portico/backend/internal/secretbox"
	"github.com/portico/backend/internal/services/connection"
	"github.com/portico/backend/internal/services/sync"
	"github.com/portico/backend/internal/services/syncjob"
	"github.com/portico/backend/internal/services/synclog"
)

// @title Portico Sync API
// @version 1.0
// @description API for managing database connections and syncing data to destinations like Typesense. List endpoints for sync jobs and sync logs return a paginated envelope: items, page, page_size, total, total_pages.
// @BasePath /
func main() {
	gdb, cfg, err := db.OpenFromEnv()
	if err != nil {
		log.Fatalf("%v", err)
	}
	if secretbox.UsingDevKey() {
		log.Printf("warning: APP_KEY is unset; connection secrets use the development key")
	}

	registry := register.DefaultRegistry()
	connSvc := connection.NewService(gdb)
	jobSvc := syncjob.NewService(gdb)
	logSvc := synclog.NewService(gdb)
	orch := sync.NewOrchestrator(gdb, registry)
	ai := llm.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL, cfg.OpenAIModel)
	h := handlers.New(connSvc, jobSvc, logSvc, orch, registry, ai)

	r := router.New(h)
	log.Printf("listening on %s", cfg.HTTPAddr)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
