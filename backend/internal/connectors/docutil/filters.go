package docutil

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
)

// MatchFilters reports whether row satisfies all filters (AND).
func MatchFilters(row map[string]any, filters []connectors.Filter) bool {
	for _, f := range filters {
		if !matchFilter(row, f) {
			return false
		}
	}
	return true
}

func matchFilter(row map[string]any, f connectors.Filter) bool {
	values := pathValues(row, strings.Split(f.Column, "."))
	switch f.Operator {
	case models.RuleOperatorIsNull:
		if len(values) == 0 {
			return true
		}
		for _, raw := range values {
			if raw == nil {
				return true
			}
		}
		return false
	case models.RuleOperatorIsNotNull:
		for _, raw := range values {
			if raw != nil {
				return true
			}
		}
		return false
	}
	if len(values) == 0 {
		return false
	}

	switch f.Operator {
	case models.RuleOperatorEq:
		for _, raw := range values {
			if raw == nil {
				continue
			}
			if fmt.Sprint(raw) == f.Value || numericEqual(raw, f.Value) {
				return true
			}
		}
		return false
	case models.RuleOperatorNeq:
		for _, raw := range values {
			if raw == nil {
				continue
			}
			if fmt.Sprint(raw) == f.Value || numericEqual(raw, f.Value) {
				return false
			}
		}
		return true
	case models.RuleOperatorLike:
		for _, raw := range values {
			if raw != nil && likeMatch(fmt.Sprint(raw), f.Value) {
				return true
			}
		}
		return false
	case models.RuleOperatorIn:
		want := splitCSV(f.Value)
		for _, raw := range values {
			if raw == nil {
				continue
			}
			left := fmt.Sprint(raw)
			for _, v := range want {
				if left == v || numericEqual(raw, v) {
					return true
				}
			}
		}
		return false
	case models.RuleOperatorNotIn:
		want := splitCSV(f.Value)
		for _, raw := range values {
			if raw == nil {
				continue
			}
			left := fmt.Sprint(raw)
			for _, v := range want {
				if left == v || numericEqual(raw, v) {
					return false
				}
			}
		}
		return true
	case models.RuleOperatorGt, models.RuleOperatorGte, models.RuleOperatorLt, models.RuleOperatorLte:
		for _, raw := range values {
			if raw == nil {
				continue
			}
			cmp, ok := compareNumeric(raw, f.Value)
			if !ok {
				cmp = strings.Compare(fmt.Sprint(raw), f.Value)
			}
			switch f.Operator {
			case models.RuleOperatorGt:
				if cmp > 0 {
					return true
				}
			case models.RuleOperatorGte:
				if cmp >= 0 {
					return true
				}
			case models.RuleOperatorLt:
				if cmp < 0 {
					return true
				}
			case models.RuleOperatorLte:
				if cmp <= 0 {
					return true
				}
			}
		}
		return false
	}
	return false
}

// pathValues collects values at a dotted path. Arrays contribute every element (Mongo-style).
func pathValues(v any, parts []string) []any {
	if len(parts) == 0 {
		if v == nil {
			return nil
		}
		return []any{v}
	}
	key := parts[0]
	rest := parts[1:]
	switch x := v.(type) {
	case map[string]any:
		child, ok := x[key]
		if !ok {
			return nil
		}
		return pathValues(child, rest)
	case []map[string]any:
		var out []any
		for _, item := range x {
			out = append(out, pathValues(item, parts)...)
		}
		return out
	case []any:
		var out []any
		for _, item := range x {
			out = append(out, pathValues(item, parts)...)
		}
		return out
	default:
		return nil
	}
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

// FilterRows returns rows that match all filters.
func FilterRows(rows []map[string]any, filters []connectors.Filter) []map[string]any {
	if len(filters) == 0 {
		return rows
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if MatchFilters(row, filters) {
			out = append(out, row)
		}
	}
	return out
}

// SortRows sorts rows by order in place. Nil or empty column is a no-op.
func SortRows(rows []map[string]any, order *connectors.Order) {
	if order == nil || strings.TrimSpace(order.Column) == "" {
		return
	}
	col := strings.TrimSpace(order.Column)
	sort.SliceStable(rows, func(i, j int) bool {
		cmp := compareValues(rows[i][col], rows[j][col])
		if order.Desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareValues(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	if cmp, ok := compareNumeric(a, fmt.Sprint(b)); ok {
		return cmp
	}
	return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
}

// PageRows returns a slice of rows for limit/offset. limit <= 0 defaults to 50.
func PageRows(rows []map[string]any, limit, offset int) []map[string]any {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		return []map[string]any{}
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end]
}

// SelectColumns keeps only the named columns when columns is non-empty.
func SelectColumns(rows []map[string]any, columns []string) []map[string]any {
	if len(columns) == 0 {
		return rows
	}
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		doc := make(map[string]any, len(columns))
		for _, c := range columns {
			if v, ok := row[c]; ok {
				doc[c] = v
			}
		}
		out[i] = doc
	}
	return out
}

// ReadChunksInMemory filters rows then invokes fn with batches of chunkSize.
func ReadChunksInMemory(rows []map[string]any, chunkSize int, filters []connectors.Filter, fn func([]map[string]any) error) error {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	filtered := FilterRows(rows, filters)
	batch := make([]map[string]any, 0, chunkSize)
	for _, row := range filtered {
		batch = append(batch, row)
		if len(batch) >= chunkSize {
			if err := fn(batch); err != nil {
				return err
			}
			batch = make([]map[string]any, 0, chunkSize)
		}
	}
	if len(batch) > 0 {
		return fn(batch)
	}
	return nil
}

// InferFieldType maps a Go value to a Portico field type.
func InferFieldType(v any) connectors.FieldType {
	switch x := v.(type) {
	case nil:
		return connectors.FieldTypeString
	case bool:
		return connectors.FieldTypeBool
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return connectors.FieldTypeInt64
	case float32, float64:
		return connectors.FieldTypeFloat64
	case []any, []map[string]any:
		return connectors.FieldTypeObjectArray
	case map[string]any:
		return connectors.FieldTypeObject
	case string:
		return connectors.FieldTypeString
	default:
		_ = x
		return connectors.FieldTypeString
	}
}
