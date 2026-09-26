package tests

import (
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/typesense"
	"github.com/portico/backend/internal/models"
)

func TestBuildFilterBy(t *testing.T) {
	got, ok := typesense.BuildFilterBy([]connectors.Filter{
		{Column: "email", Operator: models.RuleOperatorEq, Value: "user_99995@example.com"},
	})
	if !ok {
		t.Fatal("expected ok")
	}
	want := "email:=`user_99995@example.com`"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	got, ok = typesense.BuildFilterBy([]connectors.Filter{
		{Column: "status", Operator: models.RuleOperatorIn, Value: "a, b"},
		{Column: "client_id", Operator: models.RuleOperatorGt, Value: "10"},
	})
	if !ok {
		t.Fatal("expected ok")
	}
	want = "status:=[`a`, `b`] && client_id:>10"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	_, ok = typesense.BuildFilterBy([]connectors.Filter{
		{Column: "name", Operator: models.RuleOperatorLike, Value: "%ada%"},
	})
	if ok {
		t.Fatal("like should not push server-side")
	}
}
