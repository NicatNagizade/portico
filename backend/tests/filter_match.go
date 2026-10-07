package tests

import (
	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
)

// matchFilters reports whether row satisfies all filters (AND). Used by mock sources.
func matchFilters(row map[string]any, filters []connectors.Filter) bool {
	return docutil.MatchFilters(row, filters)
}
