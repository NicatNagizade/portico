package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
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
}

type Source struct {
	cfg Config
	db  *sql.DB
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
	db, err := sql.Open("mysql", dsn)
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

func (s *Source) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT COLUMN_NAME, DATA_TYPE, COLUMN_KEY
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, s.cfg.Database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schema := &connectors.TableSchema{}
	for rows.Next() {
		var name, dataType, columnKey string
		if err := rows.Scan(&name, &dataType, &columnKey); err != nil {
			return nil, err
		}
		schema.Columns = append(schema.Columns, connectors.ColumnSchema{
			Name:       name,
			Type:       mapMySQLType(dataType),
			PrimaryKey: columnKey == "PRI",
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
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(table))
	var n int64
	if err := s.db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Source) ReadChunks(ctx context.Context, table string, chunkSize int, fn func([]map[string]any) error) error {
	query := fmt.Sprintf("SELECT * FROM %s", quoteIdent(table))
	return sqlutil.ReadChunks(ctx, s.db, query, chunkSize, fn)
}

func (s *Source) QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error) {
	quotedCols := make([]string, len(columns))
	for i, c := range columns {
		quotedCols[i] = quoteIdent(c)
	}
	return sqlutil.QueryRows(
		ctx,
		s.db,
		quoteIdent(table),
		quotedCols,
		quoteIdent(whereColumn),
		whereValues,
		sqlutil.PlaceholdersMySQL,
	)
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
