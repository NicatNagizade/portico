package main

import (
	"fmt"
	"log"
	"os"

	"github.com/portico/backend/internal/db"
)

func main() {
	args := os.Args[1:]
	refresh := false
	if len(args) > 0 {
		switch args[0] {
		case "refresh":
			refresh = true
		case "migrate", "":
			// default: ensure DB + AutoMigrate via Open
		default:
			fmt.Fprintf(os.Stderr, "usage: migrate [migrate|refresh]\n")
			os.Exit(2)
		}
	}

	gdb, _, err := db.OpenFromEnv()
	if err != nil {
		log.Fatalf("%v", err)
	}

	if refresh {
		if err := db.Refresh(gdb); err != nil {
			log.Fatalf("refresh: %v", err)
		}
		log.Printf("migrated (refresh): dropped all tables and recreated schema")
		return
	}

	log.Printf("migrated: schema up to date")
}
