package mysql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/sqlutil"
	"github.com/portico/backend/internal/models"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type Source struct {
	cfg Config
	db  *gorm.DB
}

func NewSource(conn *models.Connection) (connectors.SourceReader, error) {
	var cfg Config
	if err := json.Unmarshal(conn.Config, &cfg); err != nil {
		return nil, fmt.Errorf("parse mysql config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 3306
	}
	return &Source{cfg: cfg}, nil
}

func (s *Source) Open(ctx context.Context) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		s.cfg.User, s.cfg.Password, s.cfg.Host, s.cfg.Port, s.cfg.Database)
	db, err := sqlutil.Open(ctx, mysqlDriver.Open(dsn))
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
		SELECT TABLE_NAME
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME`, s.cfg.Database).Scan(&tables).Error
	return tables, err
}

func (s *Source) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	type colRow struct {
		Name      string `gorm:"column:COLUMN_NAME"`
		DataType  string `gorm:"column:DATA_TYPE"`
		ColumnKey string `gorm:"column:COLUMN_KEY"`
	}
	var rows []colRow
	if err := s.db.WithContext(ctx).Raw(`
		SELECT COLUMN_NAME, DATA_TYPE, COLUMN_KEY
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, s.cfg.Database, table).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("table %q not found or has no columns", table)
	}
	schema := &connectors.TableSchema{}
	for _, r := range rows {
		schema.Columns = append(schema.Columns, connectors.ColumnSchema{
			Name:       r.Name,
			Type:       mapMySQLType(r.DataType),
			PrimaryKey: r.ColumnKey == "PRI",
		})
	}
	return schema, nil
}

func (s *Source) table(table string) *gorm.DB {
	return s.db.Table(quoteIdent(table))
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

func mapMySQLType(dataType string) connectors.FieldType {
	switch strings.ToLower(dataType) {
	case "json":
		return connectors.FieldTypeObject
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return connectors.FieldTypeInt64
	case "float", "double", "decimal", "numeric":
		return connectors.FieldTypeFloat64
	case "bool", "boolean":
		return connectors.FieldTypeBool
	default:
		return connectors.FieldTypeString
	}
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
