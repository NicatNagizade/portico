package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/sqlutil"
	"github.com/portico/backend/internal/models"
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
	db  *sql.DB
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
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return err
	}
	s.db = db
	return nil
}

func (s *Source) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Source) ListTables(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_type = 'BASE TABLE'
		ORDER BY table_name`, s.cfg.Schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (s *Source) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	// DISTINCT ON avoids duplicate columns when a column participates in multiple constraints.
	rows, err := s.db.QueryContext(ctx, `
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
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position`, s.cfg.Schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schema := &connectors.TableSchema{}
	for rows.Next() {
		var name, dataType string
		var isPK bool
		if err := rows.Scan(&name, &dataType, &isPK); err != nil {
			return nil, err
		}
		schema.Columns = append(schema.Columns, connectors.ColumnSchema{
			Name:       name,
			Type:       mapPostgresType(dataType),
			PrimaryKey: isPK,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(schema.Columns) == 0 {
		return nil, fmt.Errorf("table %q not found or has no columns", table)
	}
	return schema, nil
}

func (s *Source) Count(ctx context.Context, table string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", quoteIdent(s.cfg.Schema), quoteIdent(table))
	var n int64
	if err := s.db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Source) ReadChunks(ctx context.Context, table string, chunkSize int, fn func([]map[string]any) error) error {
	query := fmt.Sprintf("SELECT * FROM %s.%s", quoteIdent(s.cfg.Schema), quoteIdent(table))
	return sqlutil.ReadChunks(ctx, s.db, query, chunkSize, fn)
}

func (s *Source) QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error) {
	quotedCols := make([]string, len(columns))
	for i, c := range columns {
		quotedCols[i] = quoteIdent(c)
	}
	from := fmt.Sprintf("%s.%s", quoteIdent(s.cfg.Schema), quoteIdent(table))
	return sqlutil.QueryRows(
		ctx,
		s.db,
		from,
		quotedCols,
		quoteIdent(whereColumn),
		whereValues,
		sqlutil.PlaceholdersPostgres,
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
