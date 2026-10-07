package mysql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/sqlutil"
	"github.com/portico/backend/internal/models"
	"gorm.io/gorm"
)

type Destination struct {
	cfg    Config
	db     *gorm.DB
	schema *connectors.TableSchema // set in Prepare; drives WriteBatch columns
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
		return fmt.Errorf("mysql prepare %q: schema has no columns", name)
	}
	sql, err := BuildCreateTableSQL(name, schema)
	if err != nil {
		return err
	}
	drop := fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteIdent(name))
	if err := d.db.WithContext(ctx).Exec(drop).Error; err != nil {
		return fmt.Errorf("drop mysql table %q: %w", name, err)
	}
	if err := d.db.WithContext(ctx).Exec(sql).Error; err != nil {
		return fmt.Errorf("create mysql table %q: %w", name, err)
	}
	d.schema = schema
	return nil
}

// BuildCreateTableSQL builds a CREATE TABLE statement from a Portico schema.
func BuildCreateTableSQL(name string, schema *connectors.TableSchema) (string, error) {
	return sqlutil.CreateTableSQL(quoteIdent(name), schema, quoteIdent, MapColumnType)
}

// MapColumnType maps Portico field types to MySQL column types.
func MapColumnType(t connectors.FieldType) string {
	switch t {
	case connectors.FieldTypeInt64:
		return "BIGINT"
	case connectors.FieldTypeFloat64:
		return "DOUBLE"
	case connectors.FieldTypeBool:
		return "TINYINT(1)"
	case connectors.FieldTypeObject, connectors.FieldTypeObjectArray:
		return "JSON"
	default:
		return "TEXT"
	}
}

func (d *Destination) WriteBatch(ctx context.Context, name string, docs []map[string]any) error {
	if err := sqlutil.InsertRows(ctx, d.db, quoteIdent(name), docs, d.schema); err != nil {
		return fmt.Errorf("mysql insert into %q: %w", name, err)
	}
	return nil
}

// RowForWrite keeps schema columns and JSON-encodes object fields for MySQL JSON columns.
func RowForWrite(doc map[string]any, schema *connectors.TableSchema) map[string]any {
	return sqlutil.RowForWrite(doc, schema)
}

func (d *Destination) Query(ctx context.Context, name string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, int64, error) {
	rows, total, err := sqlutil.QueryDestination(ctx, d.db.Table(quoteIdent(name)), filters, quoteIdent, nil, limit, offset, order)
	if err != nil {
		return nil, 0, fmt.Errorf("mysql query %q: %w", name, err)
	}
	return rows, total, nil
}
