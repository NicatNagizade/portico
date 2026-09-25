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

type Source struct {
	cfg Config
	db  *gorm.DB
}

func NewSource(conn *models.Connection) (connectors.SourceReader, error) {
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
	}
	return &Source{cfg: cfg}, nil
}

func (s *Source) Open(ctx context.Context) error {
	db, err := openDB(ctx, s.cfg)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

func (s *Source) Close() error {
	return sqlutil.Close(s.db)
}

func (s *Source) ListTables(ctx context.Context) ([]string, error) {
	var tables []string
	err := s.db.WithContext(ctx).Raw(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`).Scan(&tables).Error
	return tables, err
}

func (s *Source) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	type colRow struct {
		CID       int    `gorm:"column:cid"`
		Name      string `gorm:"column:name"`
		Type      string `gorm:"column:type"`
		NotNull   int    `gorm:"column:notnull"`
		Default   any    `gorm:"column:dflt_value"`
		PrimaryKey int   `gorm:"column:pk"`
	}
	var rows []colRow
	q := fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(table))
	if err := s.db.WithContext(ctx).Raw(q).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("table %q not found or has no columns", table)
	}
	schema := &connectors.TableSchema{}
	for _, r := range rows {
		schema.Columns = append(schema.Columns, connectors.ColumnSchema{
			Name:       r.Name,
			Type:       mapSQLiteType(r.Type),
			PrimaryKey: r.PrimaryKey > 0,
		})
	}
	return schema, nil
}

func (s *Source) table(table string) *gorm.DB {
	return s.db.Table(table)
}

func (s *Source) Count(ctx context.Context, table string, filters []connectors.Filter) (int64, error) {
	return sqlutil.Count(ctx, s.table(table), filters, quoteIdent)
}

func (s *Source) ReadChunks(ctx context.Context, table string, chunkSize int, filters []connectors.Filter, fn func([]map[string]any) error) error {
	return sqlutil.ReadFilteredChunks(ctx, s.table(table), chunkSize, filters, quoteIdent, fn)
}

func (s *Source) Query(ctx context.Context, table string, columns []string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, error) {
	return sqlutil.QueryPage(ctx, s.table(table), columns, filters, quoteIdent, limit, offset, order)
}

func (s *Source) QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error) {
	return sqlutil.QueryRows(ctx, s.table(table), columns, whereColumn, whereValues, quoteIdent)
}

func mapSQLiteType(dataType string) connectors.FieldType {
	t := strings.ToUpper(strings.TrimSpace(dataType))
	switch {
	case strings.Contains(t, "INT"):
		return connectors.FieldTypeInt64
	case strings.Contains(t, "REAL"), strings.Contains(t, "FLOA"), strings.Contains(t, "DOUB"):
		return connectors.FieldTypeFloat64
	case strings.Contains(t, "BOOL"):
		return connectors.FieldTypeBool
	case strings.Contains(t, "JSON"):
		return connectors.FieldTypeObject
	default:
		return connectors.FieldTypeString
	}
}
