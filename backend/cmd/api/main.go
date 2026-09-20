package main

import (
	"log"
	"os"

	"github.com/portico/backend/internal/config"
	"github.com/portico/backend/internal/connectors/register"
	"github.com/portico/backend/internal/db"
	"github.com/portico/backend/internal/handlers"
	"github.com/portico/backend/internal/router"
	"github.com/portico/backend/internal/secretbox"
	"github.com/portico/backend/internal/services/connection"
	"github.com/portico/backend/internal/services/sync"
	"github.com/portico/backend/internal/services/syncjob"
	"github.com/portico/backend/internal/services/synclog"
)

// @title Portico Sync API
// @version 1.0
// @description API for managing database connections and syncing data to destinations like Typesense.
// @BasePath /
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	// APP_KEY stays in process memory. It is never written to the database.
	secretbox.SetKey(os.Getenv("APP_KEY"))
	if secretbox.UsingDevKey() {
		log.Printf("warning: APP_KEY is unset; connection secrets use the development key")
	}

	gdb, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	registry := register.DefaultRegistry()
	connSvc := connection.NewService(gdb)
	jobSvc := syncjob.NewService(gdb)
	logSvc := synclog.NewService(gdb)
	orch := sync.NewOrchestrator(gdb, registry)
	h := handlers.New(connSvc, jobSvc, logSvc, orch)

	r := router.New(h)
	log.Printf("listening on %s", cfg.HTTPAddr)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
