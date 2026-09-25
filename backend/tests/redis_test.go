package tests

import (
	"encoding/json"
	"testing"

	"github.com/portico/backend/internal/connectors/redis"
	"github.com/portico/backend/internal/models"
	"gorm.io/datatypes"
)

func TestRedisParseConfig(t *testing.T) {
	cfg, err := redis.ParseConfig([]byte(`{"host":"127.0.0.1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 6379 {
		t.Fatalf("port=%d", cfg.Port)
	}
	if cfg.KeySeparator != ":" {
		t.Fatalf("sep=%q", cfg.KeySeparator)
	}
	if _, err := redis.ParseConfig([]byte(`{}`)); err == nil {
		t.Fatal("expected host required")
	}
}

func TestRedisKeyHelpers(t *testing.T) {
	cfg := redis.Config{Host: "127.0.0.1", Port: 6379, KeySeparator: ":"}
	if got := cfg.DocKey("users", "42"); got != "users:42" {
		t.Fatalf("DocKey=%q", got)
	}
	if got := cfg.KeyPattern("users"); got != "users:*" {
		t.Fatalf("KeyPattern=%q", got)
	}
	if got := cfg.IDFromKey("users", "users:42"); got != "42" {
		t.Fatalf("IDFromKey=%q", got)
	}
	table, ok := cfg.TableFromKey("users:42")
	if !ok || table != "users" {
		t.Fatalf("TableFromKey=%q ok=%v", table, ok)
	}
	if _, ok := cfg.TableFromKey(redis.TablesCatalogKey); ok {
		t.Fatal("catalog key should not be a table")
	}
}

func TestRedisNewDestination(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"host": "127.0.0.1", "port": 6379, "db": 0,
	})
	dst, err := redis.NewDestination(&models.Connection{
		Type:   models.ConnectionTypeRedis,
		Config: datatypes.JSON(raw),
	})
	if err != nil {
		t.Fatal(err)
	}
	if dst == nil {
		t.Fatal("expected destination")
	}
}
