package tests

import (
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
)

func TestDocutilMatchFilters(t *testing.T) {
	row := map[string]any{"id": 1, "name": "Ada", "score": 9.5}
	if !docutil.MatchFilters(row, []connectors.Filter{
		{Column: "name", Operator: models.RuleOperatorEq, Value: "Ada"},
	}) {
		t.Fatal("eq should match")
	}
	if docutil.MatchFilters(row, []connectors.Filter{
		{Column: "name", Operator: models.RuleOperatorEq, Value: "Bob"},
	}) {
		t.Fatal("eq should not match")
	}
	filtered := docutil.FilterRows([]map[string]any{row, {"id": 2, "name": "Bob"}}, []connectors.Filter{
		{Column: "id", Operator: models.RuleOperatorGt, Value: "1"},
	})
	if len(filtered) != 1 || filtered[0]["name"] != "Bob" {
		t.Fatalf("filtered=%v", filtered)
	}
}

func TestDocutilSchemaFromDocs(t *testing.T) {
	schema := docutil.SchemaFromDocs([]map[string]any{
		{
			"id":   "1",
			"name": "Ada",
			"meta": map[string]any{"a": 1},
			"posts": []any{
				map[string]any{"title": "hello", "score": 2},
			},
		},
	})
	if len(schema.Columns) < 2 {
		t.Fatalf("columns=%v", schema.Columns)
	}
	if schema.Columns[0].Name != "id" || !schema.Columns[0].PrimaryKey {
		t.Fatalf("id should be first PK: %+v", schema.Columns[0])
	}
	byName := map[string]connectors.ColumnSchema{}
	for _, c := range schema.Columns {
		byName[c.Name] = c
	}
	meta := byName["meta"]
	if meta.Type != connectors.FieldTypeObject || len(meta.Columns) != 1 || meta.Columns[0].Name != "a" {
		t.Fatalf("meta=%+v", meta)
	}
	posts := byName["posts"]
	if posts.Type != connectors.FieldTypeObjectArray || len(posts.Columns) < 2 {
		t.Fatalf("posts=%+v", posts)
	}

	flat := docutil.FlattenColumnPaths(schema.Columns)
	names := make([]string, len(flat))
	for i, c := range flat {
		names[i] = c.Name
	}
	want := []string{"id", "meta", "meta.a", "name", "posts", "posts.score", "posts.title"}
	// order follows schema column order then nested
	for _, w := range want {
		found := false
		for _, n := range names {
			if n == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing flattened path %q in %v", w, names)
		}
	}
}

func TestDocutilMatchFiltersNestedPath(t *testing.T) {
	row := map[string]any{
		"posts": []any{
			map[string]any{"title": "hello"},
			map[string]any{"title": "world"},
		},
	}
	if !docutil.MatchFilters(row, []connectors.Filter{
		{Column: "posts.title", Operator: models.RuleOperatorEq, Value: "world"},
	}) {
		t.Fatal("nested eq should match any array element")
	}
	if docutil.MatchFilters(row, []connectors.Filter{
		{Column: "posts.title", Operator: models.RuleOperatorEq, Value: "nope"},
	}) {
		t.Fatal("nested eq should not match missing value")
	}
}
