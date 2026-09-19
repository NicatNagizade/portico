package tests

import (
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/typesense"
)

func TestBuildCollectionSchemaDefaults(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
		},
	}

	coll := typesense.BuildCollectionSchema("applicants", schema, nil)
	if coll.Name != "applicants" {
		t.Fatalf("name=%q", coll.Name)
	}
	if coll.EnableNestedFields == nil || !*coll.EnableNestedFields {
		t.Fatalf("expected enable_nested_fields=true by default, got %+v", coll.EnableNestedFields)
	}
	if coll.DefaultSortingField != nil {
		t.Fatalf("expected no default_sorting_field by default, got %q", *coll.DefaultSortingField)
	}
}

func TestBuildCollectionSchemaOverrides(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "score", Type: connectors.FieldTypeInt64},
		},
	}
	sortField := "score"
	nested := false
	cfg := &typesense.CollectionConfig{
		DefaultSortingField: &sortField,
		EnableNestedFields:  &nested,
		SymbolsToIndex:      []string{"_"},
		TokenSeparators:     []string{"-"},
	}

	coll := typesense.BuildCollectionSchema("scores", schema, cfg)
	if coll.DefaultSortingField == nil || *coll.DefaultSortingField != "score" {
		t.Fatalf("default_sorting_field=%v", coll.DefaultSortingField)
	}
	if coll.EnableNestedFields == nil || *coll.EnableNestedFields {
		t.Fatalf("expected enable_nested_fields=false, got %+v", coll.EnableNestedFields)
	}
	if coll.SymbolsToIndex == nil || len(*coll.SymbolsToIndex) != 1 || (*coll.SymbolsToIndex)[0] != "_" {
		t.Fatalf("symbols_to_index=%v", coll.SymbolsToIndex)
	}
	if coll.TokenSeparators == nil || len(*coll.TokenSeparators) != 1 || (*coll.TokenSeparators)[0] != "-" {
		t.Fatalf("token_separators=%v", coll.TokenSeparators)
	}
}

func TestParseCollectionConfig(t *testing.T) {
	cfg, err := typesense.ParseCollectionConfig(nil)
	if err != nil || cfg != nil {
		t.Fatalf("nil config: cfg=%v err=%v", cfg, err)
	}
	cfg, err = typesense.ParseCollectionConfig([]byte(`{"default_sorting_field":"id","enable_nested_fields":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultSortingField == nil || *cfg.DefaultSortingField != "id" {
		t.Fatalf("got %+v", cfg)
	}
	if cfg.EnableNestedFields == nil || *cfg.EnableNestedFields {
		t.Fatalf("got %+v", cfg.EnableNestedFields)
	}
}
