package main

import (
	"fmt"
	"log"
	"os"

	"github.com/portico/backend/internal/bootstrap"
	"github.com/portico/backend/internal/config"
	"github.com/portico/backend/internal/db"
	"github.com/portico/backend/internal/secretbox"
	"github.com/portico/backend/internal/services/connection"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	secretbox.SetKey(os.Getenv("APP_KEY"))

	gdb, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	path := os.Getenv("CONNECTIONS_FILE")
	if path == "" {
		path = "connections.json"
	}

	results, err := bootstrap.ImportConnections(connection.NewService(gdb), path)
	if err != nil {
		log.Fatalf("import: %v", err)
	}
	for _, r := range results {
		fmt.Printf("%s %s (%s) id=%d\n", r.Action, r.Name, r.Type, r.ID)
	}
	fmt.Printf("imported %d connection(s) from %s\n", len(results), path)
}
