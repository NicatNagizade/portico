package typesense

import (
	"fmt"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
)

// BuildFilterBy maps Portico filters to a Typesense filter_by clause.
// ok is false when any filter cannot be pushed server-side (caller should fall back).
// sortableID is the numeric companion of "id" (default id_int).
func BuildFilterBy(filters []connectors.Filter, sortableID string) (filterBy string, ok bool) {
	if sortableID == "" {
		sortableID = SortableIDField
	}
	if len(filters) == 0 {
		return "", true
	}
	parts := make([]string, 0, len(filters))
	for _, f := range filters {
		col := strings.TrimSpace(f.Column)
		if col == "" {
			return "", false
		}
		part, partOK := filterClause(col, f.Operator, f.Value, sortableID)
		if !partOK {
			return "", false
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " && "), true
}

func filterClause(col, op, value, sortableID string) (string, bool) {
	col = docutil.IDFilterColumn(col, op, col, sortableID)
	switch op {
	case models.RuleOperatorEq:
		return col + ":=" + quoteFilterValue(value), true
	case models.RuleOperatorNeq:
		return col + ":!=" + quoteFilterValue(value), true
	case models.RuleOperatorGt:
		return col + ":>" + value, true
	case models.RuleOperatorGte:
		return col + ":>=" + value, true
	case models.RuleOperatorLt:
		return col + ":<" + value, true
	case models.RuleOperatorLte:
		return col + ":<=" + value, true
	case models.RuleOperatorIn:
		list, err := filterValueList(value)
		if err != nil {
			return "", false
		}
		return col + ":=" + list, true
	case models.RuleOperatorNotIn:
		list, err := filterValueList(value)
		if err != nil {
			return "", false
		}
		return col + ":!=" + list, true
	default:
		// like / is_null / is_not_null have no clean Typesense equivalent
		return "", false
	}
}

func quoteFilterValue(v string) string {
	// Backticks denote a string literal (handles @, commas, spaces, etc.).
	return "`" + strings.ReplaceAll(v, "`", "") + "`"
}

func filterValueList(value string) (string, error) {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, quoteFilterValue(p))
	}
	if len(out) == 0 {
		return "", fmt.Errorf("empty filter value list")
	}
	return "[" + strings.Join(out, ", ") + "]", nil
}
