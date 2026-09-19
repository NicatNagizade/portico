package sync

import (
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

// ApplyFields renames or drops document fields according to sync job field rules.
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
			destName := destinationFieldName(rule)
			if destName == "" || destName == sourceName {
				continue
			}
			val, ok := doc[sourceName]
			if !ok {
				continue
			}
			doc[destName] = val
			delete(doc, sourceName)
		}
	}
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
