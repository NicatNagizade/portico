package tests

import (
	"encoding/json"
	"testing"

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
	got := mongodb.DocForWrite(map[string]any{"id": 42, "name": "Ada"})
	if got["_id"] != 42 {
		t.Fatalf("_id=%v, want 42", got["_id"])
	}
	if _, ok := got["id"]; ok {
		t.Fatal("id should be removed when mapped to _id")
	}
	if got["name"] != "Ada" {
		t.Fatalf("name=%v", got["name"])
	}

	oid := bson.NewObjectID()
	round := mongodb.DocFromRead(map[string]any{"_id": oid, "name": "Ada"})
	if round["id"] != oid.Hex() {
		t.Fatalf("id=%v, want hex %s", round["id"], oid.Hex())
	}
	if _, ok := round["_id"]; ok {
		t.Fatal("_id should not appear in explore docs")
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
