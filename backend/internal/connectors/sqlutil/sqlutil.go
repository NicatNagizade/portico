package sqlutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// ReadChunks runs query and invokes fn for each batch of rows.
func ReadChunks(ctx context.Context, db *sql.DB, query string, chunkSize int, fn func([]map[string]any) error) error {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	rows, err := db.QueryContext(ctx, query)
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

// QueryRows runs a SELECT with an IN filter and returns all matching rows.
// placeholderFn builds bound placeholders for n values (e.g. "?" or "$1,$2").
func QueryRows(
	ctx context.Context,
	db *sql.DB,
	fromClause string,
	columns []string,
	whereColumnQuoted string,
	whereValues []any,
	placeholderFn func(n int) string,
) ([]map[string]any, error) {
	if len(whereValues) == 0 {
		return nil, nil
	}
	selectList := "*"
	if len(columns) > 0 {
		parts := make([]string, len(columns))
		copy(parts, columns)
		selectList = strings.Join(parts, ", ")
	}
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s IN (%s)",
		selectList,
		fromClause,
		whereColumnQuoted,
		placeholderFn(len(whereValues)),
	)
	rows, err := db.QueryContext(ctx, query, whereValues...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		doc := make(map[string]any, len(cols))
		for i, col := range cols {
			doc[col] = NormalizeValue(vals[i], colTypes[i].DatabaseTypeName())
		}
		out = append(out, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// PlaceholdersMySQL returns "?,?,?" for n values.
func PlaceholdersMySQL(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

// PlaceholdersPostgres returns "$1,$2,$3" for n values.
func PlaceholdersPostgres(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(parts, ",")
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
