package sync

import (
	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
)

// ActiveFilters converts active sync job rules into source filters (AND'd).
func ActiveFilters(rules []models.SyncJobRule) []connectors.Filter {
	if len(rules) == 0 {
		return nil
	}
	out := make([]connectors.Filter, 0, len(rules))
	for _, r := range rules {
		if !r.IsActive() {
			continue
		}
		out = append(out, connectors.Filter{
			Column:   r.Field,
			Operator: r.Operator,
			Value:    r.Value,
		})
	}
	return out
}
