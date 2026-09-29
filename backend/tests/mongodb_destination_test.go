package tests

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/mongodb"
	"github.com/portico/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"gorm.io/datatypes"
)

func TestMongoParseConfigDefaults(t *testing.T) {
	cfg, err := mongodb.ParseConfig([]byte(`{"host":"localhost","database":"app"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 27017 {
		t.Fatalf("port=%d, want 27017", cfg.Port)
	}
	if cfg.AuthSource != "admin" {
		t.Fatalf("auth_source=%q, want admin", cfg.AuthSource)
	}
}

func TestMongoParseConfigRequiresHostAndDatabase(t *testing.T) {
	if _, err := mongodb.ParseConfig([]byte(`{"database":"app"}`)); err == nil {
		t.Fatal("expected host required error")
	}
	if _, err := mongodb.ParseConfig([]byte(`{"host":"localhost"}`)); err == nil {
		t.Fatal("expected database required error")
	}
}

func TestMongoBuildURI(t *testing.T) {
	uri := mongodb.BuildURI(mongodb.Config{
		Host:       "localhost",
		Port:       27017,
		User:       "user",
		Password:   "p@ss:word",
		Database:   "portico_test",
		AuthSource: "admin",
	})
	want := "mongodb://user:p%40ss%3Aword@localhost:27017/portico_test?authSource=admin&directConnection=true"
	if uri != want {
		t.Fatalf("uri=%q, want %q", uri, want)
	}

	noAuth := mongodb.BuildURI(mongodb.Config{
		Host:     "127.0.0.1",
		Port:     27017,
		Database: "db",
	})
	if noAuth != "mongodb://127.0.0.1:27017/db?directConnection=true" {
		t.Fatalf("noAuth=%q", noAuth)
	}
}

func TestMongoDocForWriteAndFromRead(t *testing.T) {
	got := mongodb.DocForWrite(map[string]any{"id": "42", "name": "Ada"})
	if got["_id"] != "42" {
		t.Fatalf("_id=%v, want string 42", got["_id"])
	}
	if got[mongodb.SortableIDField] != int64(42) {
		t.Fatalf("id_int=%v, want 42", got[mongodb.SortableIDField])
	}
	if _, ok := got["id"]; ok {
		t.Fatal("id should be removed when mapped to _id")
	}
	if got["name"] != "Ada" {
		t.Fatalf("name=%v", got["name"])
	}

	nonNumeric := mongodb.DocForWrite(map[string]any{"id": "abc", "name": "Bob"})
	if nonNumeric["_id"] != "abc" {
		t.Fatalf("_id=%v, want abc", nonNumeric["_id"])
	}
	if _, ok := nonNumeric[mongodb.SortableIDField]; ok {
		t.Fatal("id_int should be omitted for non-numeric id")
	}

	oid := bson.NewObjectID()
	round := mongodb.DocFromRead(map[string]any{
		"_id": oid, "name": "Ada", mongodb.SortableIDField: int64(1),
	})
	if round["id"] != oid.Hex() {
		t.Fatalf("id=%v, want hex %s", round["id"], oid.Hex())
	}
	if _, ok := round["_id"]; ok {
		t.Fatal("_id should not appear in explore docs")
	}
	if _, ok := round[mongodb.SortableIDField]; ok {
		t.Fatal("id_int should not appear in explore docs")
	}
}

func TestMongoNewDestination(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"host": "localhost", "port": 27017, "database": "app",
	})
	dst, err := mongodb.NewDestination(&models.Connection{
		Type:   models.ConnectionTypeMongoDB,
		Config: datatypes.JSON(raw),
	})
	if err != nil {
		t.Fatal(err)
	}
	if dst == nil {
		t.Fatal("expected destination")
	}
}

func TestMongoParseCollectionConfig(t *testing.T) {
	cfg, err := mongodb.ParseCollectionConfig(nil)
	if err != nil || cfg != nil {
		t.Fatalf("nil config: cfg=%v err=%v", cfg, err)
	}
	cfg, err = mongodb.ParseCollectionConfig([]byte(`{"apply_schema":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || !cfg.ApplySchema {
		t.Fatalf("got %+v", cfg)
	}
	cfg, err = mongodb.ParseCollectionConfig([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ApplySchema {
		t.Fatal("expected apply_schema=false by default")
	}
}

func TestMongoBuildValidator(t *testing.T) {
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
			{Name: "name", Type: connectors.FieldTypeString},
			{
				Name: "posts",
				Type: connectors.FieldTypeObjectArray,
				Columns: []connectors.ColumnSchema{
					{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
					{Name: "title", Type: connectors.FieldTypeString},
					{
						Name: "comments",
						Type: connectors.FieldTypeObjectArray,
						Columns: []connectors.ColumnSchema{
							{Name: "id", Type: connectors.FieldTypeInt64, PrimaryKey: true},
							{Name: "body", Type: connectors.FieldTypeString},
							{Name: "reactions", Type: connectors.FieldTypeObjectArray},
						},
					},
				},
			},
			{Name: "meta", Type: connectors.FieldTypeObject},
			{Name: "active", Type: connectors.FieldTypeBool},
			{Name: "score", Type: connectors.FieldTypeFloat64},
		},
	}
	validator, err := mongodb.BuildValidator(schema)
	if err != nil {
		t.Fatal(err)
	}
	js, ok := validator["$jsonSchema"].(bson.M)
	if !ok {
		t.Fatalf("expected $jsonSchema map, got %#v", validator["$jsonSchema"])
	}
	props, ok := js["properties"].(bson.M)
	if !ok {
		t.Fatalf("properties=%#v", js["properties"])
	}
	if _, hasID := props["id"]; hasID {
		t.Fatal("id should be mapped to _id only")
	}
	idProp, ok := props["_id"].(bson.M)
	if !ok {
		t.Fatalf("_id=%#v", props["_id"])
	}
	if got := fmt.Sprint(idProp["bsonType"]); got != "[string null]" {
		t.Fatalf("_id bsonType=%v", idProp["bsonType"])
	}
	idIntProp, ok := props[mongodb.SortableIDField].(bson.M)
	if !ok {
		t.Fatalf("id_int=%#v", props[mongodb.SortableIDField])
	}
	if got := fmt.Sprint(idIntProp["bsonType"]); got != "[int long null]" {
		t.Fatalf("id_int bsonType=%v", idIntProp["bsonType"])
	}
	nameProp := props["name"].(bson.M)
	if got := fmt.Sprint(nameProp["bsonType"]); got != "[string date null]" {
		t.Fatalf("name bsonType=%v", nameProp["bsonType"])
	}
	postsProp := props["posts"].(bson.M)
	if got := fmt.Sprint(postsProp["bsonType"]); got != "[array null]" {
		t.Fatalf("posts bsonType=%v", postsProp["bsonType"])
	}
	items, ok := postsProp["items"].(bson.M)
	if !ok {
		t.Fatalf("posts items=%#v", postsProp["items"])
	}
	itemProps, ok := items["properties"].(bson.M)
	if !ok {
		t.Fatalf("posts item properties=%#v", items["properties"])
	}
	title, ok := itemProps["title"].(bson.M)
	if !ok {
		t.Fatalf("expected title under posts items, got %#v", itemProps)
	}
	if got := fmt.Sprint(title["bsonType"]); got != "[string date null]" {
		t.Fatalf("posts.title bsonType=%v", title["bsonType"])
	}
	postID, ok := itemProps["id"].(bson.M)
	if !ok {
		t.Fatalf("expected nested id under posts, got %#v", itemProps)
	}
	if got := fmt.Sprint(postID["bsonType"]); got != "[int long null]" {
		t.Fatalf("posts.id bsonType=%v (nested id stays int, not _id)", postID["bsonType"])
	}
	comments, ok := itemProps["comments"].(bson.M)
	if !ok {
		t.Fatalf("expected comments under posts items, got %#v", itemProps)
	}
	commentItems, ok := comments["items"].(bson.M)
	if !ok {
		t.Fatalf("comments items=%#v", comments["items"])
	}
	commentProps := commentItems["properties"].(bson.M)
	if body, ok := commentProps["body"].(bson.M); !ok || fmt.Sprint(body["bsonType"]) != "[string date null]" {
		t.Fatalf("comments.body=%#v", commentProps["body"])
	}
	if _, ok := commentProps["reactions"]; !ok {
		t.Fatalf("expected reactions under comments, got %#v", commentProps)
	}
	if _, err := mongodb.BuildValidator(nil); err == nil {
		t.Fatal("expected error for nil schema")
	}
	if _, err := mongodb.BuildValidator(&connectors.TableSchema{}); err == nil {
		t.Fatal("expected error for empty schema")
	}

	roundTrip := mongodb.SchemaFromValidator(validator)
	if roundTrip == nil {
		t.Fatal("expected schema from validator")
	}
	rtByName := map[string]connectors.ColumnSchema{}
	for _, c := range roundTrip.Columns {
		rtByName[c.Name] = c
	}
	if rtByName["id"].Type != connectors.FieldTypeString || !rtByName["id"].PrimaryKey {
		t.Fatalf("id=%+v", rtByName["id"])
	}
	if _, ok := rtByName[mongodb.SortableIDField]; ok {
		t.Fatal("id_int should be hidden from Portico schema")
	}
	rtPosts := rtByName["posts"]
	if rtPosts.Type != connectors.FieldTypeObjectArray {
		t.Fatalf("posts=%+v", rtPosts)
	}
	rtPostFields := map[string]connectors.ColumnSchema{}
	for _, c := range rtPosts.Columns {
		rtPostFields[c.Name] = c
	}
	if rtPostFields["title"].Type != connectors.FieldTypeString {
		t.Fatalf("posts.title=%+v", rtPostFields["title"])
	}
	rtComments := rtPostFields["comments"]
	if rtComments.Type != connectors.FieldTypeObjectArray || len(rtComments.Columns) == 0 {
		t.Fatalf("posts.comments=%+v", rtComments)
	}
}
