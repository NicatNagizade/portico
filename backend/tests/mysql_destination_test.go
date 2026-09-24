package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/mysql"
	"github.com/portico/backend/internal/models"
	"gorm.io/datatypes"
)

func TestMySQLParseConfig(t *testing.T) {
	cfg, err := mysql.ParseConfig([]byte(`{"host":"localhost","database":"app"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 3306 {
		t.Fatalf("port=%d, want 3306", cfg.Port)
	}
	if _, err := mysql.ParseConfig([]byte(`{"database":"app"}`)); err == nil {
		t.Fatal("expected host required")
	}
}

func TestMySQLBuildCreateTableSQL(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
			{Name: "posts", Type: connectors.FieldTypeObjectArray},
			{Name: "meta", Type: connectors.FieldTypeObject},
			{Name: "active", Type: connectors.FieldTypeBool},
			{Name: "score", Type: connectors.FieldTypeFloat64},
		},
	}
	sql, err := mysql.BuildCreateTableSQL("users_copy", schema)
	if err != nil {
		t.Fatal(err)
	}
	wantParts := []string{
		"CREATE TABLE `users_copy`",
		"`id` BIGINT",
		"`name` TEXT",
		"`posts` JSON",
		"`meta` JSON",
		"`active` TINYINT(1)",
		"`score` DOUBLE",
		"PRIMARY KEY (`id`)",
	}
	for _, part := range wantParts {
		if !strings.Contains(sql, part) {
			t.Fatalf("sql missing %q:\n%s", part, sql)
		}
	}
}

func TestMySQLMapColumnType(t *testing.T) {
	if mysql.MapColumnType(connectors.FieldTypeString) != "TEXT" {
		t.Fatal("string")
	}
	if mysql.MapColumnType(connectors.FieldTypeObject) != "JSON" {
		t.Fatal("object")
	}
}

func TestMySQLRowForWrite(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "posts", Type: connectors.FieldTypeObjectArray},
			{Name: "extra_ignored", Type: connectors.FieldTypeString},
		},
	}
	got := mysql.RowForWrite(map[string]any{
		"id":    1,
		"posts": []map[string]any{{"title": "hi"}},
		"noise": "skip",
	}, schema)
	if got["id"] != 1 {
		t.Fatalf("id=%v", got["id"])
	}
	if _, ok := got["noise"]; ok {
		t.Fatal("noise should be dropped")
	}
	posts, ok := got["posts"].(string)
	if !ok || !strings.Contains(posts, "hi") {
		t.Fatalf("posts=%v", got["posts"])
	}
	if _, ok := got["extra_ignored"]; ok {
		t.Fatal("missing columns should stay unset")
	}
}

func TestMySQLNewDestination(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"host": "localhost", "port": 3306, "database": "app", "user": "root",
	})
	dst, err := mysql.NewDestination(&models.Connection{
		Type:   models.ConnectionTypeMySQL,
		Config: datatypes.JSON(raw),
	})
	if err != nil {
		t.Fatal(err)
	}
	if dst == nil {
		t.Fatal("expected destination")
	}
}
