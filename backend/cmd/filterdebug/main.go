package main

import (
	"fmt"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
)

func main() {
	// Simulate Typesense: numeric fields as float64, id as string
	row := map[string]any{
		"id":        "1306780",
		"client_id": float64(271),
		"status":    float64(1),
		"big":       float64(1306780), // if someone stored id as number
	}
	cases := []struct {
		col, val string
	}{
		{"client_id", "271"},
		{"status", "1"},
		{"id", "1306780"},
		{"big", "1306780"},
		{"client_id", "271 "},
		{"client_id", " 271"},
		{"status", "1.0"},
	}
	for _, c := range cases {
		ok := docutil.MatchFilters(row, []connectors.Filter{{
			Column: c.col, Operator: models.RuleOperatorEq, Value: c.val,
		}})
		fmt.Printf("%s=%q => %v (sprint big=%q)\n", c.col, c.val, ok, fmt.Sprint(row["big"]))
	}
}
