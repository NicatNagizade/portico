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
		{"id": "1", "name": "Ada", "meta": map[string]any{"a": 1}},
	})
	if len(schema.Columns) < 2 {
		t.Fatalf("columns=%v", schema.Columns)
	}
	if schema.Columns[0].Name != "id" || !schema.Columns[0].PrimaryKey {
		t.Fatalf("id should be first PK: %+v", schema.Columns[0])
	}
}
