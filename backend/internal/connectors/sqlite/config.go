package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portico/backend/internal/connectors/sqlutil"
	sqliteDriver "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Config struct {
	Path string `json:"path"`
}

func ParseConfig(raw json.RawMessage) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse sqlite config: %w", err)
	}
	if strings.TrimSpace(cfg.Path) == "" {
		return Config{}, fmt.Errorf("parse sqlite config: path is required")
	}
	return cfg, nil
}

func openDB(ctx context.Context, cfg Config) (*gorm.DB, error) {
	return sqlutil.Open(ctx, sqliteDriver.Open(cfg.Path))
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
