package migrate

import (
	"context"
	"fmt"
	"regexp"

	"github.com/portico/exampledata/internal/config"
)

var dbNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Run creates the database if missing, then creates tables and indexes.
// When truncate is true, existing example tables are dropped first.
func Run(ctx context.Context, cfg *config.Config, truncate bool) error {
	if !dbNamePattern.MatchString(cfg.DBName) {
		return fmt.Errorf("invalid database name %q", cfg.DBName)
	}
	if cfg.IsMySQL() {
		return runMySQL(ctx, cfg, truncate)
	}
	return runPostgres(ctx, cfg, truncate)
}
