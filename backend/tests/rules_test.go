package tests

import (
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/sqlutil"
	"github.com/portico/backend/internal/models"
	syncsvc "github.com/portico/backend/internal/services/sync"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func quoteSQLite(name string) string {
	return `"` + name + `"`
}

func openFilterDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE items (client_id INTEGER, status TEXT, deleted_at TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO items (client_id, status, deleted_at) VALUES
		(123, 'a', NULL),
		(999, 'b', 'x'),
		(123, 'b', NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestApplyFiltersEq(t *testing.T) {
	db := openFilterDB(t)
	q, err := sqlutil.ApplyFilters(
		db.Table("items"),
		[]connectors.Filter{{Column: "client_id", Operator: models.RuleOperatorEq, Value: "123"}},
		quoteSQLite,
	)
	if err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("count=%d want 2", n)
	}
}

func TestApplyFiltersInAndNull(t *testing.T) {
	db := openFilterDB(t)
	q, err := sqlutil.ApplyFilters(
		db.Table("items"),
		[]connectors.Filter{
			{Column: "status", Operator: models.RuleOperatorIn, Value: "a, b"},
			{Column: "deleted_at", Operator: models.RuleOperatorIsNull},
		},
		quoteSQLite,
	)
	if err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("count=%d want 2", n)
	}
}

func TestMatchFilters(t *testing.T) {
	row := map[string]any{"client_id": int64(123), "name": "Ann"}
	if !matchFilters(row, []connectors.Filter{
		{Column: "client_id", Operator: models.RuleOperatorEq, Value: "123"},
	}) {
		t.Fatal("expected match")
	}
	if matchFilters(row, []connectors.Filter{
		{Column: "client_id", Operator: models.RuleOperatorEq, Value: "999"},
	}) {
		t.Fatal("expected miss")
	}
}

func TestActiveFiltersSkipsInactive(t *testing.T) {
	active := true
	inactive := false
	filters := syncsvc.ActiveFilters([]models.SyncJobRule{
		{Field: "client_id", Operator: models.RuleOperatorEq, Value: "123", Active: &active},
		{Field: "status", Operator: models.RuleOperatorEq, Value: "x", Active: &inactive},
	})
	if len(filters) != 1 || filters[0].Column != "client_id" {
		t.Fatalf("got %+v", filters)
	}
}
