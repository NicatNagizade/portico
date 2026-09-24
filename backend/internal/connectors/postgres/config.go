package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/portico/backend/internal/connectors/sqlutil"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ParseConfig decodes Postgres connection config JSON.
func ParseConfig(raw json.RawMessage) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse postgres config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	if cfg.Schema == "" {
		cfg.Schema = "public"
	}
	if cfg.Host == "" {
		return Config{}, fmt.Errorf("parse postgres config: host is required")
	}
	if cfg.Database == "" {
		return Config{}, fmt.Errorf("parse postgres config: database is required")
	}
	return cfg, nil
}

func (c Config) dsn() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
}

func openDB(ctx context.Context, cfg Config) (*gorm.DB, error) {
	return sqlutil.Open(ctx, postgresDriver.Open(cfg.dsn()))
}
