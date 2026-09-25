package tests

import (
	"strings"
	"testing"

	"github.com/portico/backend/internal/connectors/register"
	"github.com/portico/backend/internal/models"
	"gorm.io/datatypes"
)

func TestRegistryBidirectional(t *testing.T) {
	r := register.DefaultRegistry()
	cases := []struct {
		typ    string
		config string
	}{
		{models.ConnectionTypeMySQL, `{"host":"localhost","database":"app","user":"root"}`},
		{models.ConnectionTypePostgres, `{"host":"localhost","database":"app","user":"postgres"}`},
		{models.ConnectionTypeSQLite, `{"path":":memory:"}`},
		{models.ConnectionTypeMongoDB, `{"host":"127.0.0.1","database":"app"}`},
		{models.ConnectionTypeTypesense, `{"host":"localhost","api_key":"xyz"}`},
		{models.ConnectionTypeRedis, `{"host":"127.0.0.1"}`},
	}
	for _, tc := range cases {
		conn := &models.Connection{
			Type:   tc.typ,
			Config: datatypes.JSON([]byte(tc.config)),
		}
		src, err := r.NewSource(conn)
		if err != nil {
			t.Fatalf("%s NewSource: %v", tc.typ, err)
		}
		if src == nil {
			t.Fatalf("%s NewSource returned nil", tc.typ)
		}
		dst, err := r.NewDestination(conn)
		if err != nil {
			t.Fatalf("%s NewDestination: %v", tc.typ, err)
		}
		if dst == nil {
			t.Fatalf("%s NewDestination returned nil", tc.typ)
		}
	}

	_, err := r.NewSource(&models.Connection{Type: "unknown", Config: datatypes.JSON([]byte(`{}`))})
	if err == nil || !strings.Contains(err.Error(), "no source connector") {
		t.Fatalf("expected missing source error, got %v", err)
	}
}
