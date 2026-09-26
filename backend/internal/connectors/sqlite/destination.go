package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/sqlutil"
	"github.com/portico/backend/internal/models"
	"gorm.io/gorm"
)

type Destination struct {
	cfg    Config
	db     *gorm.DB
	schema *connectors.TableSchema
}

func NewDestination(conn *models.Connection) (connectors.DestinationWriter, error) {
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
	}
	return &Destination{cfg: cfg}, nil
}

func (d *Destination) Open(ctx context.Context) error {
	db, err := openDB(ctx, d.cfg)
	if err != nil {
		return err
	}
	d.db = db
	return nil
}

func (d *Destination) Close() error {
	err := sqlutil.Close(d.db)
	d.db = nil
	d.schema = nil
	return err
}

func (d *Destination) Prepare(ctx context.Context, name string, schema *connectors.TableSchema, _ json.RawMessage) error {
	if schema == nil || len(schema.Columns) == 0 {
		return fmt.Errorf("sqlite prepare %q: schema has no columns", name)
	}
	sql, err := BuildCreateTableSQL(name, schema)
	if err != nil {
		return err
	}
	drop := fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteIdent(name))
	if err := d.db.WithContext(ctx).Exec(drop).Error; err != nil {
		return fmt.Errorf("drop sqlite table %q: %w", name, err)
	}
	if err := d.db.WithContext(ctx).Exec(sql).Error; err != nil {
		return fmt.Errorf("create sqlite table %q: %w", name, err)
	}
	d.schema = schema
	return nil
}

// BuildCreateTableSQL builds a CREATE TABLE statement from a Portico schema.
func BuildCreateTableSQL(name string, schema *connectors.TableSchema) (string, error) {
	if schema == nil || len(schema.Columns) == 0 {
		return "", fmt.Errorf("schema has no columns")
	}
	cols := make([]string, 0, len(schema.Columns)+1)
	var pks []string
	for _, col := range schema.Columns {
		if strings.TrimSpace(col.Name) == "" {
			continue
		}
		cols = append(cols, fmt.Sprintf("%s %s", quoteIdent(col.Name), MapColumnType(col.Type)))
		if col.PrimaryKey {
			pks = append(pks, quoteIdent(col.Name))
		}
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("schema has no valid columns")
	}
	if len(pks) > 0 {
		cols = append(cols, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pks, ", ")))
	}
	return fmt.Sprintf("CREATE TABLE %s (%s)", quoteIdent(name), strings.Join(cols, ", ")), nil
}

// MapColumnType maps Portico field types to SQLite column types.
func MapColumnType(t connectors.FieldType) string {
	switch t {
	case connectors.FieldTypeInt64:
		return "INTEGER"
	case connectors.FieldTypeFloat64:
		return "REAL"
	case connectors.FieldTypeBool:
		return "INTEGER"
	case connectors.FieldTypeObject, connectors.FieldTypeObjectArray:
		return "TEXT"
	default:
		return "TEXT"
	}
}

func (d *Destination) WriteBatch(ctx context.Context, name string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}
	payload := make([]map[string]any, len(docs))
	for i, doc := range docs {
		payload[i] = RowForWrite(doc, d.schema)
	}
	if err := d.db.WithContext(ctx).Table(name).Create(payload).Error; err != nil {
		return fmt.Errorf("sqlite insert into %q: %w", name, err)
	}
	return nil
}

// RowForWrite keeps schema columns and JSON-encodes object fields as TEXT.
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

func (d *Destination) Query(ctx context.Context, name string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, int64, error) {
	table := d.db.Table(name)
	total, err := sqlutil.Count(ctx, table, filters, quoteIdent)
	if err != nil {
		return nil, 0, fmt.Errorf("sqlite count %q: %w", name, err)
	}
	rows, err := sqlutil.QueryPage(ctx, table, nil, filters, quoteIdent, limit, offset, order)
	if err != nil {
		return nil, 0, fmt.Errorf("sqlite select %q: %w", name, err)
	}
	return rows, total, nil
}
