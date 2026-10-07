package tests

import (
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/mongodb"
	"github.com/portico/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoBuildFilter(t *testing.T) {
	got, err := mongodb.BuildFilter([]connectors.Filter{
		{Column: "email", Operator: models.RuleOperatorEq, Value: "user@example.com"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	want := bson.D{{Key: "email", Value: "user@example.com"}}
	if !bsonEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}

	got, err = mongodb.BuildFilter([]connectors.Filter{
		{Column: "id", Operator: models.RuleOperatorEq, Value: "42"},
		{Column: "status", Operator: models.RuleOperatorIn, Value: "a, b"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	want = bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "_id", Value: "42"}}, // document _id is always string
		bson.D{{Key: "status", Value: bson.D{{Key: "$in", Value: []any{"a", "b"}}}}},
	}}}
	if !bsonEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}

	got, err = mongodb.BuildFilter([]connectors.Filter{
		{Column: "id", Operator: models.RuleOperatorLt, Value: "10"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	want = bson.D{{Key: mongodb.SortableIDField, Value: bson.D{{Key: "$lt", Value: int64(10)}}}}
	if !bsonEqual(got, want) {
		t.Fatalf("id range got %#v want %#v", got, want)
	}

	got, err = mongodb.BuildFilter([]connectors.Filter{
		{Column: "name", Operator: models.RuleOperatorLike, Value: "%ada%"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	want = bson.D{{Key: "name", Value: bson.D{{Key: "$regex", Value: "^.*ada.*$"}}}}
	if !bsonEqual(got, want) {
		t.Fatalf("like got %#v want %#v", got, want)
	}

	got, err = mongodb.BuildFilter([]connectors.Filter{
		{Column: "deleted_at", Operator: models.RuleOperatorIsNull},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	want = bson.D{{Key: "deleted_at", Value: nil}}
	if !bsonEqual(got, want) {
		t.Fatalf("is_null got %#v want %#v", got, want)
	}

	// Nested relation paths (explore filters on destination docs).
	got, err = mongodb.BuildFilter([]connectors.Filter{
		{Column: "posts.title", Operator: models.RuleOperatorEq, Value: "hello"},
		{Column: "posts.comments.body", Operator: models.RuleOperatorLike, Value: "%hi%"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	want = bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "posts.title", Value: "hello"}},
		bson.D{{Key: "posts.comments.body", Value: bson.D{{Key: "$regex", Value: "^.*hi.*$"}}}},
	}}}
	if !bsonEqual(got, want) {
		t.Fatalf("nested path got %#v want %#v", got, want)
	}

	if _, err := mongodb.BuildFilter([]connectors.Filter{
		{Column: "", Operator: models.RuleOperatorEq, Value: "x"},
	}, ""); err == nil {
		t.Fatal("expected empty field error")
	}
}

func bsonEqual(a, b bson.D) bool {
	rawA, errA := bson.Marshal(a)
	rawB, errB := bson.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(rawA) == string(rawB)
}
