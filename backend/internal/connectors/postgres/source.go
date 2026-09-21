package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/sqlutil"
	"github.com/portico/backend/internal/models"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"sslmode"`
	Schema   string `json:"schema"`
}

type Source struct {
	cfg Config
	db  *gorm.DB
}

func NewSource(conn *models.Connection) (connectors.SourceReader, error) {
	var cfg Config
	if err := json.Unmarshal(conn.Config, &cfg); err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	if cfg.Schema == "" {
		cfg.Schema = "public"
	}
	return &Source{cfg: cfg}, nil
}

func (s *Source) Open(ctx context.Context) error {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		s.cfg.Host, s.cfg.Port, s.cfg.User, s.cfg.Password, s.cfg.Database, s.cfg.SSLMode)
	db, err := gorm.Open(postgresDriver.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return err
	}
	s.db = db
	return nil
}

func (s *Source) Close() error {
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *Source) ListTables(ctx context.Context) ([]string, error) {
	var tables []string
	err := s.db.WithContext(ctx).Raw(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE'
		ORDER BY table_name`, s.cfg.Schema).Scan(&tables).Error
	return tables, err
}

func (s *Source) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	type colRow struct {
		Name     string `gorm:"column:column_name"`
		DataType string `gorm:"column:data_type"`
		IsPK     bool   `gorm:"column:is_pk"`
	}
	var rows []colRow
	// DISTINCT ON avoids duplicate columns when a column participates in multiple constraints.
	if err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (c.ordinal_position)
			c.column_name,
			c.data_type,
			EXISTS (
				SELECT 1
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
					ON tc.constraint_name = kcu.constraint_name
					AND tc.table_schema = kcu.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY'
					AND tc.table_schema = c.table_schema
					AND tc.table_name = c.table_name
					AND kcu.column_name = c.column_name
			) AS is_pk
		FROM information_schema.columns c
		WHERE c.table_schema = ? AND c.table_name = ?
		ORDER BY c.ordinal_position`, s.cfg.Schema, table).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("table %q not found or has no columns", table)
	}
	schema := &connectors.TableSchema{}
	for _, r := range rows {
		schema.Columns = append(schema.Columns, connectors.ColumnSchema{
			Name:       r.Name,
			Type:       mapPostgresType(r.DataType),
			PrimaryKey: r.IsPK,
		})
	}
	return schema, nil
}

func (s *Source) tableRef(table string) string {
	return quoteIdent(s.cfg.Schema) + "." + quoteIdent(table)
}

func (s *Source) Count(ctx context.Context, table string, filters []connectors.Filter) (int64, error) {
	q, err := sqlutil.ApplyFilters(s.db.WithContext(ctx).Table(s.tableRef(table)), filters, quoteIdent)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Source) ReadChunks(ctx context.Context, table string, chunkSize int, filters []connectors.Filter, fn func([]map[string]any) error) error {
	q, err := sqlutil.ApplyFilters(s.db.Table(s.tableRef(table)), filters, quoteIdent)
	if err != nil {
		return err
	}
	return sqlutil.ReadChunks(ctx, q, chunkSize, fn)
}

func (s *Source) QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error) {
	quotedCols := make([]string, len(columns))
	for i, c := range columns {
		quotedCols[i] = quoteIdent(c)
	}
	return sqlutil.QueryRows(
		ctx,
		s.db.Table(s.tableRef(table)),
		quotedCols,
		quoteIdent(whereColumn),
		whereValues,
	)
}

func mapPostgresType(dataType string) connectors.FieldType {
	switch strings.ToLower(dataType) {
	case "json", "jsonb":
		return connectors.FieldTypeObject
	case "smallint", "integer", "bigint", "int2", "int4", "int8":
		return connectors.FieldTypeInt64
	case "real", "double precision", "numeric", "decimal", "float4", "float8":
		return connectors.FieldTypeFloat64
	case "boolean", "bool":
		return connectors.FieldTypeBool
	default:
		return connectors.FieldTypeString
	}
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
