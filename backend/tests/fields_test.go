package tests

import (
	"reflect"
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
	syncsvc "github.com/portico/backend/internal/services/sync"
)

func boolPtr(v bool) *bool { return &v }

func TestSchemaWithFieldsEmpty(t *testing.T) {
	base := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
		},
	}
	got := syncsvc.SchemaWithFields(base, nil)
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("expected unchanged schema, got %+v", got)
	}
}

func TestSchemaWithFieldsRenameTypeAndInactive(t *testing.T) {
	base := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "full_name", Type: connectors.FieldTypeString},
			{Name: "score", Type: connectors.FieldTypeInt64},
			{Name: "internal_notes", Type: connectors.FieldTypeString},
		},
	}
	fields := []models.SyncJobField{
		{SourceName: "full_name", DestinationName: "name", Active: boolPtr(true)},
		{SourceName: "score", DestinationType: string(connectors.FieldTypeFloat64), Active: boolPtr(true)},
		{SourceName: "internal_notes", DestinationName: "notes", Active: boolPtr(false)},
	}
	got := syncsvc.SchemaWithFields(base, fields)
	want := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
			{Name: "score", Type: connectors.FieldTypeFloat64},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SchemaWithFields mismatch\ngot  %+v\nwant %+v", got, want)
	}
}

func TestApplyFields(t *testing.T) {
	docs := []map[string]any{
		{"id": "1", "full_name": "Ada", "score": 10, "internal_notes": "secret"},
		{"id": "2", "full_name": "Grace", "score": 20, "internal_notes": "private"},
	}
	fields := []models.SyncJobField{
		{SourceName: "full_name", DestinationName: "name", Active: boolPtr(true)},
		{SourceName: "score", DestinationName: "points", Active: boolPtr(false)},
	}
	syncsvc.ApplyFields(docs, fields)

	want := []map[string]any{
		{"id": "1", "name": "Ada", "internal_notes": "secret"},
		{"id": "2", "name": "Grace", "internal_notes": "private"},
	}
	if !reflect.DeepEqual(docs, want) {
		t.Fatalf("ApplyFields mismatch\ngot  %+v\nwant %+v", docs, want)
	}
}

func TestApplyFieldsCoerceDestinationType(t *testing.T) {
	docs := []map[string]any{
		{"id": "1", "form": 42, "meta": map[string]any{"a": 1}, "score": "3.5"},
	}
	fields := []models.SyncJobField{
		{SourceName: "form", DestinationType: string(connectors.FieldTypeString), Active: boolPtr(true)},
		{SourceName: "meta", DestinationType: string(connectors.FieldTypeString), Active: boolPtr(true)},
		{SourceName: "score", DestinationType: string(connectors.FieldTypeFloat64), Active: boolPtr(true)},
	}
	syncsvc.ApplyFields(docs, fields)

	want := []map[string]any{
		{"id": "1", "form": "42", "meta": `{"a":1}`, "score": 3.5},
	}
	if !reflect.DeepEqual(docs, want) {
		t.Fatalf("ApplyFields coerce mismatch\ngot  %+v\nwant %+v", docs, want)
	}
}

func TestApplyFieldsValueMaps(t *testing.T) {
	docs := []map[string]any{
		{"id": "1", "status": 1},
		{"id": "2", "status": int64(2)},
		{"id": "3", "status": 9},
	}
	fields := []models.SyncJobField{
		{
			SourceName:      "status",
			DestinationType: string(connectors.FieldTypeString),
			Active:          boolPtr(true),
			Values: []models.SyncJobFieldValue{
				{SourceValue: "1", DestinationValue: "success"},
				{SourceValue: "2", DestinationValue: "failed"},
			},
		},
	}
	syncsvc.ApplyFields(docs, fields)

	want := []map[string]any{
		{"id": "1", "status": "success"},
		{"id": "2", "status": "failed"},
		{"id": "3", "status": "9"},
	}
	if !reflect.DeepEqual(docs, want) {
		t.Fatalf("ApplyFields value maps mismatch\ngot  %+v\nwant %+v", docs, want)
	}
}

func TestApplyFieldsNoop(t *testing.T) {
	docs := []map[string]any{{"id": "1", "name": "Ada"}}
	syncsvc.ApplyFields(docs, nil)
	if !reflect.DeepEqual(docs, []map[string]any{{"id": "1", "name": "Ada"}}) {
		t.Fatalf("expected noop, got %+v", docs)
	}
}
