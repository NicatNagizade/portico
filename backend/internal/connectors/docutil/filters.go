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

// SchemaFromDocs builds a TableSchema from sample documents. id is marked primary key when present.
func SchemaFromDocs(docs []map[string]any) *connectors.TableSchema {
	seen := map[string]connectors.FieldType{}
	var order []string
	for _, doc := range docs {
		for k, v := range doc {
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = InferFieldType(v)
			order = append(order, k)
		}
	}
	sort.Strings(order)
	// Prefer id first when present.
	if _, ok := seen["id"]; ok {
		rest := make([]string, 0, len(order)-1)
		for _, k := range order {
			if k != "id" {
				rest = append(rest, k)
			}
		}
		order = append([]string{"id"}, rest...)
	}
	schema := &connectors.TableSchema{Columns: make([]connectors.ColumnSchema, 0, len(order))}
	for _, k := range order {
		schema.Columns = append(schema.Columns, connectors.ColumnSchema{
			Name:       k,
			Type:       seen[k],
			PrimaryKey: k == "id",
		})
	}
	return schema
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
	case []any:
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
