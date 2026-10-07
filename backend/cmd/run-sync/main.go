package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"

	"github.com/portico/backend/internal/connectors/register"
	"github.com/portico/backend/internal/db"
	"github.com/portico/backend/internal/services/sync"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: run-sync <sync-job-id>\n")
		os.Exit(2)
	}
	id, err := strconv.ParseUint(os.Args[1], 10, 64)
	if err != nil || id == 0 {
		fmt.Fprintf(os.Stderr, "usage: run-sync <sync-job-id>\n")
		os.Exit(2)
	}

	gdb, _, err := db.OpenFromEnv()
	if err != nil {
		log.Fatalf("%v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	entry, runErr := sync.NewOrchestrator(gdb, register.DefaultRegistry()).Run(ctx, uint(id))
	if entry == nil {
		log.Fatalf("run sync job %d: %v", id, runErr)
	}

	var rowsTotal, rowsSynced, duration int64
	if entry.RowsTotal != nil {
		rowsTotal = *entry.RowsTotal
	}
	if entry.RowsSynced != nil {
		rowsSynced = *entry.RowsSynced
	}
	if entry.DurationMs != nil {
		duration = *entry.DurationMs
	}
	fmt.Printf("sync job %d log=%d status=%s rows=%d/%d duration_ms=%d\n%s\n",
		id, entry.ID, entry.Status, rowsSynced, rowsTotal, duration, entry.Message)
	if runErr != nil {
		os.Exit(1)
	}
}
