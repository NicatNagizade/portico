package tests

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/mysql"
	"github.com/portico/backend/internal/models"
	syncsvc "github.com/portico/backend/internal/services/sync"
)

func TestParsePrimaryKeyConfig(t *testing.T) {
	cfg, err := connectors.ParsePrimaryKeyConfig(nil)
	if err != nil || cfg.Configured || cfg.Destination != "id" || cfg.Int != "id_int" {
		t.Fatalf("defaults: %+v err=%v", cfg, err)
	}

	cfg, err = connectors.ParsePrimaryKeyConfig(json.RawMessage(`{"primary_key":" uid "}`))
	if err != nil || !cfg.Configured || !reflect.DeepEqual(cfg.Source, []string{"uid"}) || cfg.Int != "" {
		t.Fatalf("string: %+v err=%v", cfg, err)
	}

	cfg, err = connectors.ParsePrimaryKeyConfig(json.RawMessage(`{"primary_key":["user_id","post_id"]}`))
	if err != nil || !reflect.DeepEqual(cfg.Source, []string{"user_id", "post_id"}) {
		t.Fatalf("array: %+v err=%v", cfg, err)
	}

	cfg, err = connectors.ParsePrimaryKeyConfig(json.RawMessage(`{
		"primary_key":{"source":["user_id","post_id"],"destination":"doc_id","int":"doc_id_int"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Configured || cfg.Destination != "doc_id" || cfg.Int != "doc_id_int" {
		t.Fatalf("object: %+v", cfg)
	}
	if !reflect.DeepEqual(cfg.Source, []string{"user_id", "post_id"}) {
		t.Fatalf("object source: %+v", cfg.Source)
	}
}

func TestApplyPrimaryKeyOverride(t *testing.T) {
	base := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "user_id", Type: connectors.FieldTypeInt64},
			{Name: "post_id", Type: connectors.FieldTypeInt64},
			{Name: "name", Type: connectors.FieldTypeString},
		},
	}
	got, err := connectors.ApplyPrimaryKeyOverride(base, connectors.PrimaryKeyConfig{})
	if err != nil || got != base {
		t.Fatalf("empty override should be no-op")
	}

	got, err = connectors.ApplyPrimaryKeyOverride(base, connectors.PrimaryKeyConfig{
		Configured: true,
		Source:     []string{"user_id", "post_id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Columns[0].PrimaryKey || !got.Columns[1].PrimaryKey || !got.Columns[2].PrimaryKey {
		t.Fatalf("expected user_id+post_id as PKs, got %+v", got.Columns)
	}

	_, err = connectors.ApplyPrimaryKeyOverride(base, connectors.PrimaryKeyConfig{
		Configured: true,
		Source:     []string{"missing"},
	})
	if err == nil {
		t.Fatal("expected error for unknown column")
	}
}

func TestEnsureIDCompositeAndDestination(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "user_id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "post_id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
		},
	}
	docs := []map[string]any{{"user_id": 1, "post_id": 2}}
	connectors.EnsureID(docs, schema, 0, "id")
	if docs[0]["id"] != "1_2" {
		t.Fatalf("got id=%v", docs[0]["id"])
	}

	docs = []map[string]any{{"user_id": 1, "post_id": 2}}
	connectors.EnsureID(docs, schema, 0, "doc_id")
	if docs[0]["doc_id"] != "1_2" || docs[0]["id"] != "1_2" {
		t.Fatalf("got %+v", docs[0])
	}
}

func TestEffectiveSourceSchemaThroughDestination(t *testing.T) {
	base := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "uid", Type: connectors.FieldTypeInt64},
			{Name: "name", Type: connectors.FieldTypeString},
		},
	}
	schema, err := syncsvc.EffectiveSourceSchema(base, json.RawMessage(`{"primary_key":"uid"}`))
	if err != nil {
		t.Fatal(err)
	}
	fields := []models.SyncJobField{
		{SourceName: "uid", DestinationName: "user_id", Active: boolPtr(true)},
	}
	out := syncsvc.SchemaWithFields(schema, fields)
	out, err = syncsvc.EffectiveDestinationSchema(out, json.RawMessage(`{"primary_key":{"source":"uid","destination":"id"}}`))
	if err != nil {
		t.Fatal(err)
	}

	byName := map[string]connectors.ColumnSchema{}
	for _, col := range out.Columns {
		byName[col.Name] = col
	}
	if !byName["id"].PrimaryKey || byName["user_id"].PrimaryKey {
		t.Fatalf("expected id as sole dest PK, got %+v", out.Columns)
	}

	sql, err := mysql.BuildCreateTableSQL("users", out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "PRIMARY KEY (`id`)") {
		t.Fatalf("SQL missing PRIMARY KEY on id\ngot %s", sql)
	}

	docs := []map[string]any{{"uid": 42, "name": "Ada"}}
	connectors.EnsureID(docs, schema, 0, "id")
	if docs[0]["id"] != "42" {
		t.Fatalf("EnsureID from overridden PK: got %v", docs[0]["id"])
	}
}
