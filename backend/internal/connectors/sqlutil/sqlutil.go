package sqlutil

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open opens a GORM DB with a silent logger and pings it.
func Open(ctx context.Context, dialector gorm.Dialector) (*gorm.DB, error) {
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Close closes the underlying SQL DB when present.
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Count applies filters and returns row count.
func Count(ctx context.Context, db *gorm.DB, filters []connectors.Filter, quoteIdent func(string) string) (int64, error) {
	q, err := ApplyFilters(db.WithContext(ctx), filters, quoteIdent)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// ReadChunks streams rows from a prepared GORM query and invokes fn per batch.
func ReadChunks(ctx context.Context, db *gorm.DB, chunkSize int, fn func([]map[string]any) error) error {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	rows, err := db.WithContext(ctx).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	batch := make([]map[string]any, 0, chunkSize)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		doc := make(map[string]any, len(cols))
		for i, col := range cols {
			doc[col] = NormalizeValue(vals[i], colTypes[i].DatabaseTypeName())
		}
		batch = append(batch, doc)
		if len(batch) >= chunkSize {
			if err := fn(batch); err != nil {
				return err
			}
			batch = make([]map[string]any, 0, chunkSize)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(batch) > 0 {
		return fn(batch)
	}
	return nil
}

// ReadFilteredChunks applies filters then streams rows in chunks.
func ReadFilteredChunks(
	ctx context.Context,
	db *gorm.DB,
	chunkSize int,
	filters []connectors.Filter,
	quoteIdent func(string) string,
	fn func([]map[string]any) error,
) error {
	q, err := ApplyFilters(db, filters, quoteIdent)
	if err != nil {
		return err
	}
	return ReadChunks(ctx, q, chunkSize, fn)
}

// QueryRows runs a SELECT with an IN filter and returns all matching rows.
// columns and whereColumn are unquoted identifiers; quoteIdent is applied.
func QueryRows(
	ctx context.Context,
	db *gorm.DB,
	columns []string,
	whereColumn string,
	whereValues []any,
	quoteIdent func(string) string,
) ([]map[string]any, error) {
	if len(whereValues) == 0 {
		return nil, nil
	}
	q := db.WithContext(ctx)
	if len(columns) > 0 {
		quoted := make([]string, len(columns))
		for i, c := range columns {
			quoted[i] = quoteIdent(c)
		}
		q = q.Select(strings.Join(quoted, ", "))
	}
	var out []map[string]any
	if err := q.Where(quoteIdent(whereColumn)+" IN ?", whereValues).Find(&out).Error; err != nil {
		return nil, err
	}
	normalizeMaps(out)
	return out, nil
}

// QueryPage selects rows with optional column list, filters, limit, offset, and order.
// Empty columns selects all columns. limit <= 0 defaults to 50.
func QueryPage(
	ctx context.Context,
	db *gorm.DB,
	columns []string,
	filters []connectors.Filter,
	quoteIdent func(string) string,
	limit, offset int,
	order *connectors.Order,
) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q, err := ApplyFilters(db.WithContext(ctx), filters, quoteIdent)
	if err != nil {
		return nil, err
	}
	if len(columns) > 0 {
		quoted := make([]string, len(columns))
		for i, c := range columns {
			quoted[i] = quoteIdent(c)
		}
		q = q.Select(strings.Join(quoted, ", "))
	}
	if order != nil && strings.TrimSpace(order.Column) != "" {
		col := quoteIdent(strings.TrimSpace(order.Column))
		dir := " ASC"
		if order.Desc {
			dir = " DESC"
		}
		q = q.Order(col + dir)
	}
	var out []map[string]any
	if err := q.Limit(limit).Offset(offset).Find(&out).Error; err != nil {
		return nil, err
	}
	normalizeMaps(out)
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

func normalizeMaps(rows []map[string]any) {
	for _, row := range rows {
		for k, v := range row {
			row[k] = NormalizeValue(v, "")
		}
	}
}

// NormalizeValue converts driver values into JSON-friendly Go types.
func NormalizeValue(v any, dbType string) any {
	if v == nil {
		return nil
	}
	b, ok := v.([]byte)
	if !ok {
		return v
	}
	s := string(b)
	upper := strings.ToUpper(dbType)
	if strings.Contains(upper, "JSON") || (len(s) > 0 && (s[0] == '{' || s[0] == '[')) {
		var decoded any
		if err := json.Unmarshal(b, &decoded); err == nil {
			return decoded
		}
	}
	return s
}
