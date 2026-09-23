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

	coll, err := typesense.BuildCollectionSchema("applicants", schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if coll.Name != "applicants" {
		t.Fatalf("name=%q", coll.Name)
	}
	if coll.EnableNestedFields == nil || !*coll.EnableNestedFields {
		t.Fatalf("expected enable_nested_fields=true by default, got %+v", coll.EnableNestedFields)
	}
	if coll.DefaultSortingField != nil {
		t.Fatalf("expected no default_sorting_field by default, got %q", *coll.DefaultSortingField)
	}

	var hasName, hasIDInt bool
	for _, f := range coll.Fields {
		if f.Name == "id" {
			t.Fatalf("document id must not be declared in schema fields, got %+v", f)
		}
		if f.Name == "name" {
			hasName = true
		}
		if f.Name == typesense.SortableIDField {
			hasIDInt = true
			if f.Type != "int64" {
				t.Fatalf("id_int type=%q", f.Type)
			}
			if f.Sort == nil || !*f.Sort {
				t.Fatal("expected id_int sort=true")
			}
		}
	}
	if !hasName || !hasIDInt {
		t.Fatalf("expected name + id_int fields, got %+v", coll.Fields)
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

	coll, err := typesense.BuildCollectionSchema("scores", schema, cfg)
	if err != nil {
		t.Fatal(err)
	}
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

func TestBuildCollectionSchemaMapsIDSortToIDInt(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
		},
	}
	id := "id"
	coll, err := typesense.BuildCollectionSchema("applicants", schema, &typesense.CollectionConfig{
		DefaultSortingField: &id,
	})
	if err != nil {
		t.Fatal(err)
	}
	if coll.DefaultSortingField == nil || *coll.DefaultSortingField != typesense.SortableIDField {
		t.Fatalf("expected default_sorting_field=%s, got %v", typesense.SortableIDField, coll.DefaultSortingField)
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
