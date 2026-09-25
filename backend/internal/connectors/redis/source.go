package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	goredis "github.com/redis/go-redis/v9"
)

type Source struct {
	cfg    Config
	client *goredis.Client
}

func NewSource(conn *models.Connection) (connectors.SourceReader, error) {
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
	}
	return &Source{cfg: cfg}, nil
}

func (s *Source) Open(ctx context.Context) error {
	client := goredis.NewClient(&goredis.Options{
		Addr:     s.cfg.Addr(),
		Password: s.cfg.Password,
		DB:       s.cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return fmt.Errorf("redis ping: %w", err)
	}
	s.client = client
	return nil
}

func (s *Source) Close() error {
	if s.client == nil {
		return nil
	}
	err := s.client.Close()
	s.client = nil
	return err
}

func (s *Source) ListTables(ctx context.Context) ([]string, error) {
	seen := map[string]struct{}{}
	members, err := s.client.SMembers(ctx, TablesCatalogKey).Result()
	if err != nil && err != goredis.Nil {
		return nil, fmt.Errorf("redis catalog: %w", err)
	}
	for _, m := range members {
		seen[m] = struct{}{}
	}

	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, "*", 200).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan tables: %w", err)
		}
		for _, key := range keys {
			if table, ok := s.cfg.TableFromKey(key); ok {
				seen[table] = struct{}{}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	tables := make([]string, 0, len(seen))
	for t := range seen {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	return tables, nil
}

func (s *Source) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	docs, err := loadTableDocs(ctx, s.client, s.cfg, table)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeString, PrimaryKey: true},
			},
		}, nil
	}
	sample := docs
	if len(sample) > 50 {
		sample = sample[:50]
	}
	return docutil.SchemaFromDocs(sample), nil
}

func (s *Source) Count(ctx context.Context, table string, filters []connectors.Filter) (int64, error) {
	docs, err := loadTableDocs(ctx, s.client, s.cfg, table)
	if err != nil {
		return 0, err
	}
	return int64(len(docutil.FilterRows(docs, filters))), nil
}

func (s *Source) ReadChunks(ctx context.Context, table string, chunkSize int, filters []connectors.Filter, fn func([]map[string]any) error) error {
	docs, err := loadTableDocs(ctx, s.client, s.cfg, table)
	if err != nil {
		return err
	}
	return docutil.ReadChunksInMemory(docs, chunkSize, filters, fn)
}

func (s *Source) Query(ctx context.Context, table string, columns []string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, error) {
	docs, err := loadTableDocs(ctx, s.client, s.cfg, table)
	if err != nil {
		return nil, err
	}
	docs = docutil.FilterRows(docs, filters)
	docutil.SortRows(docs, order)
	docs = docutil.PageRows(docs, limit, offset)
	return docutil.SelectColumns(docs, columns), nil
}

func (s *Source) QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error) {
	if len(whereValues) == 0 {
		return nil, nil
	}
	docs, err := loadTableDocs(ctx, s.client, s.cfg, table)
	if err != nil {
		return nil, err
	}
	want := map[string]struct{}{}
	for _, v := range whereValues {
		want[fmt.Sprint(v)] = struct{}{}
	}
	out := make([]map[string]any, 0)
	for _, doc := range docs {
		if _, ok := want[fmt.Sprint(doc[whereColumn])]; ok {
			out = append(out, doc)
		}
	}
	return docutil.SelectColumns(out, columns), nil
}
