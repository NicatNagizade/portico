package tests

import (
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
	syncsvc "github.com/portico/backend/internal/services/sync"
)

func TestSingularize(t *testing.T) {
	cases := map[string]string{
		"applicants": "applicant",
		"tags":       "tag",
		"user":       "user",
		"s":          "s",
	}
	for in, want := range cases {
		if got := syncsvc.Singularize(in); got != want {
			t.Fatalf("Singularize(%q)=%q want %q", in, got, want)
		}
	}
}

func TestResolveRelationKeys(t *testing.T) {
	rel := syncsvc.ResolveRelationKeys("applicants", models.SyncJobRelation{
		Name:  "tags",
		Table: "tags",
	})
	if rel.ForeignKey != "applicant_id" || rel.RelatedKey != "tag_id" {
		t.Fatalf("unexpected defaults: %+v", rel)
	}

	rel = syncsvc.ResolveRelationKeys("applicants", models.SyncJobRelation{
		Table:      "tags",
		ForeignKey: "app_id",
		RelatedKey: "label_id",
	})
	if rel.ForeignKey != "app_id" || rel.RelatedKey != "label_id" {
		t.Fatalf("expected explicit keys preserved: %+v", rel)
	}
}

func TestAssembleBelongsToMany(t *testing.T) {
	pivot := []map[string]any{
		{"applicant_id": 1, "tag_id": 10},
		{"applicant_id": 1, "tag_id": 11},
		{"applicant_id": 2, "tag_id": 10},
	}
	related := []map[string]any{
		{"id": 10, "name": "vip"},
		{"id": 11, "name": "referral"},
	}
	grouped := syncsvc.AssembleBelongsToMany(pivot, related, "applicant_id", "tag_id", "id")
	if len(grouped["1"]) != 2 {
		t.Fatalf("parent 1: want 2 tags, got %d", len(grouped["1"]))
	}
	if len(grouped["2"]) != 1 || grouped["2"][0]["name"] != "vip" {
		t.Fatalf("parent 2: unexpected %#v", grouped["2"])
	}
}

func TestAssembleHasMany(t *testing.T) {
	children := []map[string]any{
		{"id": 10, "user_id": 1, "title": "a"},
		{"id": 11, "user_id": 1, "title": "b"},
		{"id": 12, "user_id": 2, "title": "c"},
	}
	grouped := syncsvc.AssembleHasMany(children, "user_id")
	if len(grouped["1"]) != 2 {
		t.Fatalf("user 1: want 2 posts, got %d", len(grouped["1"]))
	}
	if len(grouped["2"]) != 1 || grouped["2"][0]["title"] != "c" {
		t.Fatalf("user 2: unexpected %#v", grouped["2"])
	}
}

func TestSchemaWithRelationsSkipsInactive(t *testing.T) {
	falseVal := false
	trueVal := true
	base := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
		},
	}
	got := syncsvc.SchemaWithRelations(base, []models.SyncJobRelation{
		{Name: "tags", Type: models.RelationTypeBelongsToMany, Active: &trueVal},
		{Name: "skills", Type: models.RelationTypeBelongsToMany, Active: &falseVal},
		{Name: "comments", Type: models.RelationTypeHasMany, ParentRelation: "posts", Active: &trueVal},
	})
	if len(got.Columns) != 2 || got.Columns[1].Name != "tags" {
		t.Fatalf("expected only active root relation in schema, got %+v", got.Columns)
	}
}

func TestSchemaWithRelationsIncludesHasManyRoot(t *testing.T) {
	trueVal := true
	base := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
		},
	}
	got := syncsvc.SchemaWithRelations(base, []models.SyncJobRelation{
		{Name: "posts", Type: models.RelationTypeHasMany, Active: &trueVal},
	})
	if len(got.Columns) != 2 || got.Columns[1].Name != "posts" || got.Columns[1].Type != connectors.FieldTypeObjectArray {
		t.Fatalf("expected posts object_array, got %+v", got.Columns)
	}
}

