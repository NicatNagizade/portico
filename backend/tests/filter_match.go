package tests

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
)

// matchFilters reports whether row satisfies all filters (AND). Used by mock sources.
func matchFilters(row map[string]any, filters []connectors.Filter) bool {
	for _, f := range filters {
		if !matchFilter(row, f) {
			return false
		}
	}
	return true
}

func matchFilter(row map[string]any, f connectors.Filter) bool {
	raw, ok := row[f.Column]
	switch f.Operator {
	case models.RuleOperatorIsNull:
		return !ok || raw == nil
	case models.RuleOperatorIsNotNull:
		return ok && raw != nil
	}
	if !ok || raw == nil {
		return false
	}

	left := fmt.Sprint(raw)
	switch f.Operator {
	case models.RuleOperatorEq:
		return left == f.Value || numericEqual(raw, f.Value)
	case models.RuleOperatorNeq:
		return left != f.Value && !numericEqual(raw, f.Value)
	case models.RuleOperatorLike:
		return likeMatch(left, f.Value)
	case models.RuleOperatorIn:
		for _, v := range splitCSV(f.Value) {
			if left == v || numericEqual(raw, v) {
				return true
			}
		}
		return false
	case models.RuleOperatorNotIn:
		for _, v := range splitCSV(f.Value) {
			if left == v || numericEqual(raw, v) {
				return false
			}
		}
		return true
	case models.RuleOperatorGt, models.RuleOperatorGte, models.RuleOperatorLt, models.RuleOperatorLte:
		cmp, ok := compareNumeric(raw, f.Value)
		if !ok {
			cmp = strings.Compare(left, f.Value)
		}
		switch f.Operator {
		case models.RuleOperatorGt:
			return cmp > 0
		case models.RuleOperatorGte:
			return cmp >= 0
		case models.RuleOperatorLt:
			return cmp < 0
		case models.RuleOperatorLte:
			return cmp <= 0
		}
	}
	return false
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func numericEqual(raw any, want string) bool {
	cmp, ok := compareNumeric(raw, want)
	return ok && cmp == 0
}

func compareNumeric(raw any, want string) (int, bool) {
	left, err := strconv.ParseFloat(fmt.Sprint(raw), 64)
	if err != nil {
		return 0, false
	}
	right, err := strconv.ParseFloat(want, 64)
	if err != nil {
		return 0, false
	}
	switch {
	case left < right:
		return -1, true
	case left > right:
		return 1, true
	default:
		return 0, true
	}
}

func likeMatch(value, pattern string) bool {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '%':
			b.WriteString(".*")
		case '_':
			b.WriteString(".")
		case '\\':
			if i+1 < len(pattern) {
				i++
				b.WriteString(regexp.QuoteMeta(string(pattern[i])))
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(value)
}
