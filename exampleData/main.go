package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/portico/exampledata/internal/config"
	"github.com/portico/exampledata/internal/migrate"
	"github.com/portico/exampledata/internal/seed"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("exampledata: ")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch os.Args[1] {
	case "migrate":
		if err := migrate.Run(ctx, cfg); err != nil {
			log.Fatalf("migrate failed: %v", err)
		}
	case "seed":
		if err := seed.Run(ctx, cfg); err != nil {
			log.Fatalf("seed failed: %v", err)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  go run . migrate   Create the Postgres database (if missing) and tables
  go run . seed      Truncate and insert fake users, posts, comments, reactions

Config is read from .env (see .env.example).
`)
}
