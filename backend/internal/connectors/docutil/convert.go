package docutil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/portico/backend/internal/models"
)

// ParseInt64 parses v as a base-10 int64 (nil / non-numeric → false).
func ParseInt64(v any) (int64, bool) {
	if v == nil {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(v)), 10, 64)
	return n, err == nil
}

// IDFilterColumn maps Portico "id" for filters: range ops use rangeCol, others use eqCol.
func IDFilterColumn(col, op, eqCol, rangeCol string) string {
	col = strings.TrimSpace(col)
	if col != "id" {
		return col
	}
	switch op {
	case models.RuleOperatorGt, models.RuleOperatorGte, models.RuleOperatorLt, models.RuleOperatorLte:
		return rangeCol
	default:
		if eq := strings.TrimSpace(eqCol); eq != "" {
			return eq
		}
		return col
	}
}

// SortColumn maps Portico "id" onto a numeric companion field for ORDER BY / sort_by.
func SortColumn(col, sortableID string) string {
	col = strings.TrimSpace(col)
	if col == "id" {
		return sortableID
	}
	return col
}
