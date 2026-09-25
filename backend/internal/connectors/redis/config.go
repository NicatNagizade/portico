package redis

import (
	"encoding/json"
	"fmt"
	"strings"
)

const TablesCatalogKey = "__portico:tables"

type Config struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Password      string `json:"password"`
	DB            int    `json:"db"`
	KeySeparator  string `json:"key_separator"`
}

func ParseConfig(raw json.RawMessage) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse redis config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 6379
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return Config{}, fmt.Errorf("parse redis config: host is required")
	}
	if cfg.KeySeparator == "" {
		cfg.KeySeparator = ":"
	}
	return cfg, nil
}

func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c Config) DocKey(table, id string) string {
	return table + c.KeySeparator + id
}

func (c Config) KeyPattern(table string) string {
	return table + c.KeySeparator + "*"
}

func (c Config) IDFromKey(table, key string) string {
	prefix := table + c.KeySeparator
	if strings.HasPrefix(key, prefix) {
		return strings.TrimPrefix(key, prefix)
	}
	return key
}

func (c Config) TableFromKey(key string) (string, bool) {
	if key == TablesCatalogKey || strings.HasPrefix(key, "__portico") {
		return "", false
	}
	idx := strings.Index(key, c.KeySeparator)
	if idx <= 0 {
		return "", false
	}
	return key[:idx], true
}
