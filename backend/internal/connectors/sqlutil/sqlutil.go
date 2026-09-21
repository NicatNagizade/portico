package sqlutil

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"
)

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

// QueryRows runs a SELECT with an IN filter and returns all matching rows.
func QueryRows(
	ctx context.Context,
	db *gorm.DB,
	columns []string,
	whereColumn string,
	whereValues []any,
) ([]map[string]any, error) {
	if len(whereValues) == 0 {
		return nil, nil
	}
	q := db.WithContext(ctx)
	if len(columns) > 0 {
		q = q.Select(strings.Join(columns, ", "))
	}
	var out []map[string]any
	if err := q.Where(whereColumn+" IN ?", whereValues).Find(&out).Error; err != nil {
		return nil, err
	}
	for _, row := range out {
		for k, v := range row {
			row[k] = NormalizeValue(v, "")
		}
	}
	return out, nil
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
