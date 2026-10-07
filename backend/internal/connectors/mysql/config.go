package mysql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/portico/backend/internal/connectors/sqlutil"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ParseConfig decodes MySQL connection config JSON.
func ParseConfig(raw json.RawMessage) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse mysql config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 3306
	}
	if cfg.Host == "" {
		return Config{}, fmt.Errorf("parse mysql config: host is required")
	}
	if cfg.Database == "" {
		return Config{}, fmt.Errorf("parse mysql config: database is required")
	}
	return cfg, nil
}

func (c Config) dsn() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

func openDB(ctx context.Context, cfg Config) (*gorm.DB, error) {
	return sqlutil.Open(ctx, mysqlDriver.Open(cfg.dsn()))
}
