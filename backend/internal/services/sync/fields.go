package sync

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
)

// SchemaWithFields applies field rename/type/active rules to a table schema.
func SchemaWithFields(base *connectors.TableSchema, fields []models.SyncJobField) *connectors.TableSchema {
	if len(fields) == 0 || base == nil {
		return base
	}
	rules := fieldRulesBySource(fields)
	if len(rules) == 0 {
		return base
	}

	out := &connectors.TableSchema{
		Columns: make([]connectors.ColumnSchema, 0, len(base.Columns)),
	}
	for _, col := range base.Columns {
		rule, ok := rules[col.Name]
		if !ok {
			out.Columns = append(out.Columns, col)
			continue
		}
		if !rule.IsActive() {
			continue
		}
		mapped := col
		if name := destinationFieldName(rule); name != "" {
			mapped.Name = name
		}
		if rule.DestinationType != "" {
			mapped.Type = connectors.FieldType(rule.DestinationType)
		}
		out.Columns = append(out.Columns, mapped)
	}
	return out
}

// ApplyFields renames, drops, or type-coerces document fields according to sync job field rules.
func ApplyFields(docs []map[string]any, fields []models.SyncJobField) {
	rules := fieldRulesBySource(fields)
	if len(rules) == 0 {
		return
	}
	for _, doc := range docs {
		for sourceName, rule := range rules {
			if !rule.IsActive() {
				delete(doc, sourceName)
				continue
			}
			val, ok := doc[sourceName]
			if !ok {
				continue
			}
			if rule.DestinationType != "" {
				val = coerceValue(val, connectors.FieldType(rule.DestinationType))
			}
			destName := destinationFieldName(rule)
			doc[destName] = val
			if destName != sourceName {
				delete(doc, sourceName)
			}
		}
	}
}

func coerceValue(v any, t connectors.FieldType) any {
	if v == nil {
		return nil
	}
	switch t {
	case connectors.FieldTypeString:
		switch x := v.(type) {
		case string:
			return x
		case []byte:
			return string(x)
		case map[string]any, []any, []map[string]any:
			if b, err := json.Marshal(x); err == nil {
				return string(b)
			}
		}
		return fmt.Sprint(v)

	case connectors.FieldTypeInt64:
		if n, err := strconv.ParseInt(fmt.Sprint(v), 10, 64); err == nil {
			return n
		}

	case connectors.FieldTypeFloat64:
		if f, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
			return f
		}

	case connectors.FieldTypeBool:
		if b, err := strconv.ParseBool(fmt.Sprint(v)); err == nil {
			return b
		}

	case connectors.FieldTypeObject:
		var out map[string]any
		if unmarshalJSON(v, &out) {
			return out
		}

	case connectors.FieldTypeObjectArray:
		var out []map[string]any
		if unmarshalJSON(v, &out) {
			return out
		}
	}
	return v
}

func unmarshalJSON(v any, dest any) bool {
	var b []byte
	switch x := v.(type) {
	case string:
		b = []byte(x)
	case []byte:
		b = x
	default:
		return false
	}
	return json.Unmarshal(b, dest) == nil
}

func fieldRulesBySource(fields []models.SyncJobField) map[string]models.SyncJobField {
	out := make(map[string]models.SyncJobField)
	for _, f := range fields {
		if f.SourceName == "" {
			continue
		}
		out[f.SourceName] = f
	}
	return out
}

func destinationFieldName(f models.SyncJobField) string {
	if f.DestinationName != "" {
		return f.DestinationName
	}
	return f.SourceName
}
