package sqlutil

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"gorm.io/gorm"
)

// QuoteIdent wraps name in mark and escapes embedded marks by doubling them.
func QuoteIdent(name, mark string) string {
	return mark + strings.ReplaceAll(name, mark, mark+mark) + mark
}

// CreateTableSQL builds CREATE TABLE quotedTable (cols).
// quotedTable is already quoted and may include a schema prefix.
func CreateTableSQL(quotedTable string, schema *connectors.TableSchema, quote func(string) string, mapType func(connectors.FieldType) string) (string, error) {
	if schema == nil || len(schema.Columns) == 0 {
		return "", fmt.Errorf("schema has no columns")
	}
	cols := make([]string, 0, len(schema.Columns)+1)
	var pks []string
	for _, col := range schema.Columns {
		if strings.TrimSpace(col.Name) == "" {
			continue
		}
		cols = append(cols, fmt.Sprintf("%s %s", quote(col.Name), mapType(col.Type)))
		if col.PrimaryKey {
			pks = append(pks, quote(col.Name))
		}
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("schema has no valid columns")
	}
	if len(pks) > 0 {
		cols = append(cols, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pks, ", ")))
	}
	return fmt.Sprintf("CREATE TABLE %s (%s)", quotedTable, strings.Join(cols, ", ")), nil
}

// RowForWrite keeps schema columns and JSON-encodes object fields.
func RowForWrite(doc map[string]any, schema *connectors.TableSchema) map[string]any {
	if schema == nil || len(schema.Columns) == 0 {
		return encodeJSONValues(doc, nil)
	}
	out := make(map[string]any, len(schema.Columns))
	types := make(map[string]connectors.FieldType, len(schema.Columns))
	for _, col := range schema.Columns {
		types[col.Name] = col.Type
		if v, ok := doc[col.Name]; ok {
			out[col.Name] = v
		}
	}
	return encodeJSONValues(out, types)
}

func encodeJSONValues(doc map[string]any, types map[string]connectors.FieldType) map[string]any {
	out := make(map[string]any, len(doc))
	for k, v := range doc {
		if types != nil {
			switch types[k] {
			case connectors.FieldTypeObject, connectors.FieldTypeObjectArray:
				out[k] = mustJSON(v)
				continue
			}
		} else {
			switch v.(type) {
			case map[string]any, []any, []map[string]any:
				out[k] = mustJSON(v)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func mustJSON(v any) any {
	if v == nil {
		return nil
	}
	switch v.(type) {
	case string, []byte:
		return v
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// InsertRows writes docs with RowForWrite. Empty docs are a no-op.
func InsertRows(ctx context.Context, db *gorm.DB, table string, docs []map[string]any, schema *connectors.TableSchema) error {
	if len(docs) == 0 {
		return nil
	}
	payload := make([]map[string]any, len(docs))
	for i, doc := range docs {
		payload[i] = RowForWrite(doc, schema)
	}
	return db.WithContext(ctx).Table(table).Create(payload).Error
}
