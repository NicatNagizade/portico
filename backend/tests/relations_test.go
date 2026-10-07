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
		Type:  models.RelationTypeBelongsToMany,
		Table: "tags",
	})
	if rel.ForeignKey != "applicant_id" || rel.RelatedKey != "tag_id" {
		t.Fatalf("unexpected belongs_to_many defaults: %+v", rel)
	}

	rel = syncsvc.ResolveRelationKeys("applicants", models.SyncJobRelation{
		Type:       models.RelationTypeBelongsToMany,
		Table:      "tags",
		ForeignKey: "app_id",
		RelatedKey: "label_id",
	})
	if rel.ForeignKey != "app_id" || rel.RelatedKey != "label_id" {
		t.Fatalf("expected explicit keys preserved: %+v", rel)
	}

	rel = syncsvc.ResolveRelationKeys("users", models.SyncJobRelation{
		Type:  models.RelationTypeHasMany,
		Table: "posts",
	})
	if rel.ForeignKey != "user_id" || rel.RelatedKey != "" {
		t.Fatalf("has_many should default fk only: %+v", rel)
	}

	rel = syncsvc.ResolveRelationKeys("posts", models.SyncJobRelation{
		Type:  models.RelationTypeBelongsTo,
		Table: "users",
	})
	if rel.ForeignKey != "user_id" || rel.RelatedKey != "" {
		t.Fatalf("belongs_to should default fk from related table: %+v", rel)
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
		{ID: 1, Name: "tags", Type: models.RelationTypeBelongsToMany, Active: &trueVal},
		{ID: 2, Name: "skills", Type: models.RelationTypeBelongsToMany, Active: &falseVal},
		{ID: 3, Name: "comments", Type: models.RelationTypeHasMany, ParentID: uintPtr(1), Active: &trueVal},
	}, nil)
	if len(got.Columns) != 2 || got.Columns[1].Name != "tags" {
		t.Fatalf("expected only active root relation in schema, got %+v", got.Columns)
	}
	if len(got.Columns[1].Columns) != 1 || got.Columns[1].Columns[0].Name != "comments" {
		t.Fatalf("expected comments nested under tags, got %+v", got.Columns[1].Columns)
	}
}

func TestSchemaWithRelationsNestsChildren(t *testing.T) {
	trueVal := true
	base := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
		},
	}
	got := syncsvc.SchemaWithRelations(base, []models.SyncJobRelation{
		{ID: 1, Name: "posts", Type: models.RelationTypeHasMany, Active: &trueVal},
		{ID: 2, Name: "comments", Type: models.RelationTypeHasMany, ParentID: uintPtr(1), Active: &trueVal},
		{ID: 3, Name: "reactions", Type: models.RelationTypeHasMany, ParentID: uintPtr(2), Active: &trueVal},
	}, nil)
	if len(got.Columns) != 2 || got.Columns[1].Name != "posts" {
		t.Fatalf("expected posts root, got %+v", got.Columns)
	}
	posts := got.Columns[1]
	if len(posts.Columns) != 1 || posts.Columns[0].Name != "comments" {
		t.Fatalf("expected comments under posts, got %+v", posts.Columns)
	}
	comments := posts.Columns[0]
	if len(comments.Columns) != 1 || comments.Columns[0].Name != "reactions" {
		t.Fatalf("expected reactions under comments, got %+v", comments.Columns)
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
	}, nil)
	if len(got.Columns) != 2 || got.Columns[1].Name != "posts" || got.Columns[1].Type != connectors.FieldTypeObjectArray {
		t.Fatalf("expected posts object_array, got %+v", got.Columns)
	}
}

func TestSchemaWithRelationsIncludesHasOneAsObject(t *testing.T) {
	trueVal := true
	base := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
		},
	}
	got := syncsvc.SchemaWithRelations(base, []models.SyncJobRelation{
		{Name: "profile", Type: models.RelationTypeHasOne, Active: &trueVal},
		{Name: "company", Type: models.RelationTypeBelongsTo, Active: &trueVal},
	}, nil)
	if len(got.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %+v", got.Columns)
	}
	if got.Columns[1].Type != connectors.FieldTypeObject || got.Columns[2].Type != connectors.FieldTypeObject {
		t.Fatalf("expected has_one/belongs_to as object, got %+v", got.Columns)
	}
}

func TestSchemaWithRelationsIncludesRelatedFields(t *testing.T) {
	trueVal := true
	base := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
		},
	}
	related := map[string]*connectors.TableSchema{
		"posts": {
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "title", Type: connectors.FieldTypeString},
				{Name: "score", Type: connectors.FieldTypeFloat64},
			},
		},
		"comments": {
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
				{Name: "body", Type: connectors.FieldTypeString},
			},
		},
	}
	got := syncsvc.SchemaWithRelations(base, []models.SyncJobRelation{
		{
			ID: 1, Name: "posts", Type: models.RelationTypeHasMany, Table: "posts", Active: &trueVal,
			Fields: []models.SyncJobField{
				{SourceName: "score", DestinationType: string(connectors.FieldTypeInt64), Active: &trueVal},
			},
		},
		{ID: 2, Name: "comments", Type: models.RelationTypeHasMany, Table: "comments", ParentID: uintPtr(1), Active: &trueVal},
	}, related)

	posts := got.Columns[1]
	if len(posts.Columns) < 4 {
		t.Fatalf("expected posts fields + comments relation, got %+v", posts.Columns)
	}
	byName := map[string]connectors.ColumnSchema{}
	for _, c := range posts.Columns {
		byName[c.Name] = c
	}
	if byName["title"].Type != connectors.FieldTypeString {
		t.Fatalf("title=%+v", byName["title"])
	}
	if byName["score"].Type != connectors.FieldTypeInt64 {
		t.Fatalf("score should use field override type, got %+v", byName["score"])
	}
	comments, ok := byName["comments"]
	if !ok || comments.Type != connectors.FieldTypeObjectArray {
		t.Fatalf("comments relation=%+v", comments)
	}
	commentFields := map[string]connectors.FieldType{}
	for _, c := range comments.Columns {
		commentFields[c.Name] = c.Type
	}
	if commentFields["body"] != connectors.FieldTypeString || commentFields["id"] != connectors.FieldTypeInt64 {
		t.Fatalf("comment fields=%v", commentFields)
	}
}

func uintPtr(v uint) *uint { return &v }
