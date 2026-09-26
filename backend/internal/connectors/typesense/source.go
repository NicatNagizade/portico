package typesense

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	"github.com/typesense/typesense-go/v2/typesense"
	"github.com/typesense/typesense-go/v2/typesense/api"
	"github.com/typesense/typesense-go/v2/typesense/api/pointer"
)

type Source struct {
	cfg    Config
	client *typesense.Client
}

func NewSource(conn *models.Connection) (connectors.SourceReader, error) {
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
	}
	return &Source{cfg: cfg}, nil
}

// ParseConfig decodes Typesense connection config JSON.
func ParseConfig(raw json.RawMessage) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse typesense config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 8108
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "http"
	}
	if cfg.Host == "" {
		return Config{}, fmt.Errorf("parse typesense config: host is required")
	}
	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("parse typesense config: api_key is required")
	}
	return cfg, nil
}

func (s *Source) Open(ctx context.Context) error {
	node := fmt.Sprintf("%s://%s:%d", s.cfg.Protocol, s.cfg.Host, s.cfg.Port)
	s.client = typesense.NewClient(
		typesense.WithNodes([]string{node}),
		typesense.WithAPIKey(s.cfg.APIKey),
	)
	if _, err := s.client.Health(ctx, 5*time.Second); err != nil {
		return fmt.Errorf("typesense health check: %w", err)
	}
	return nil
}

func (s *Source) Close() error {
	s.client = nil
	return nil
}

func (s *Source) ListTables(ctx context.Context) ([]string, error) {
	colls, err := s.client.Collections().Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("typesense list collections: %w", err)
	}
	names := make([]string, 0, len(colls))
	for _, c := range colls {
		if c.Name != "" {
			names = append(names, c.Name)
		}
	}
	return names, nil
}

func (s *Source) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	coll, err := s.client.Collection(table).Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("typesense retrieve collection %q: %w", table, err)
	}
	schema := &connectors.TableSchema{
		Columns: []connectors.ColumnSchema{
			{Name: "id", Type: connectors.FieldTypeString, PrimaryKey: true},
		},
	}
	for _, f := range coll.Fields {
		if f.Name == "" || f.Name == "id" || f.Name == SortableIDField {
			continue
		}
		schema.Columns = append(schema.Columns, connectors.ColumnSchema{
			Name: f.Name,
			Type: mapTypesenseType(f.Type),
		})
	}
	return schema, nil
}

func mapTypesenseType(t string) connectors.FieldType {
	switch t {
	case "int32", "int64":
		return connectors.FieldTypeInt64
	case "float":
		return connectors.FieldTypeFloat64
	case "bool":
		return connectors.FieldTypeBool
	case "object":
		return connectors.FieldTypeObject
	case "object[]":
		return connectors.FieldTypeObjectArray
	default:
		return connectors.FieldTypeString
	}
}

func (s *Source) Count(ctx context.Context, table string, filters []connectors.Filter) (int64, error) {
	filterBy, ok := BuildFilterBy(filters)
	if len(filters) > 0 && !ok {
		docs, err := loadAll(ctx, s.client, table)
		if err != nil {
			return 0, err
		}
		return int64(len(docutil.FilterRows(docs, filters))), nil
	}
	_, total, err := searchPage(ctx, s.client, table, filterBy, 1, 0)
	return total, err
}

func (s *Source) ReadChunks(ctx context.Context, table string, chunkSize int, filters []connectors.Filter, fn func([]map[string]any) error) error {
	docs, err := loadAll(ctx, s.client, table)
	if err != nil {
		return err
	}
	return docutil.ReadChunksInMemory(docs, chunkSize, filters, fn)
}

func (s *Source) Query(ctx context.Context, table string, columns []string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, error) {
	filterBy, ok := BuildFilterBy(filters)
	// Full scan when filters can't be pushed, or when sorting (needs global order before page).
	if order != nil || (len(filters) > 0 && !ok) {
		docs, err := loadAll(ctx, s.client, table)
		if err != nil {
			return nil, err
		}
		docs = docutil.FilterRows(docs, filters)
		docutil.SortRows(docs, order)
		docs = docutil.PageRows(docs, limit, offset)
		return docutil.SelectColumns(docs, columns), nil
	}
	docs, _, err := searchPage(ctx, s.client, table, filterBy, limit, offset)
	if err != nil {
		return nil, err
	}
	return docutil.SelectColumns(docs, columns), nil
}

func (s *Source) QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error) {
	if len(whereValues) == 0 {
		return nil, nil
	}
	docs, err := loadAll(ctx, s.client, table)
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

// searchPage runs q=* with optional filter_by and returns one page plus total found.
func searchPage(ctx context.Context, client *typesense.Client, table, filterBy string, limit, offset int) ([]map[string]any, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	coll, err := client.Collection(table).Retrieve(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("typesense retrieve collection %q: %w", table, err)
	}
	queryBy := searchableQueryBy(coll.Fields)
	if queryBy == "" {
		return []map[string]any{}, 0, nil
	}
	params := &api.SearchCollectionParams{
		Q:       pointer.String("*"),
		QueryBy: pointer.String(queryBy),
		Page:    pointer.Int(offset/limit + 1),
		PerPage: pointer.Int(limit),
	}
	if filterBy != "" {
		params.FilterBy = pointer.String(filterBy)
	}
	result, err := client.Collection(table).Documents().Search(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("typesense search %q: %w", table, err)
	}
	var total int64
	if result.Found != nil {
		total = int64(*result.Found)
	}
	rows := []map[string]any{}
	if result.Hits == nil {
		return rows, total, nil
	}
	for _, hit := range *result.Hits {
		if hit.Document == nil {
			continue
		}
		doc := make(map[string]any, len(*hit.Document))
		for k, v := range *hit.Document {
			if k == SortableIDField {
				continue
			}
			doc[k] = v
		}
		rows = append(rows, doc)
	}
	return rows, total, nil
}

func loadAll(ctx context.Context, client *typesense.Client, table string) ([]map[string]any, error) {
	coll, err := client.Collection(table).Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("typesense retrieve collection %q: %w", table, err)
	}
	queryBy := searchableQueryBy(coll.Fields)
	if queryBy == "" {
		// No searchable string fields — try exporting via document retrieve is not available;
		// return empty rather than fail hard when collection has only non-string fields.
		return []map[string]any{}, nil
	}

	const perPage = 250
	page := 1
	rows := []map[string]any{}
	for {
		params := &api.SearchCollectionParams{
			Q:       pointer.String("*"),
			QueryBy: pointer.String(queryBy),
			Page:    pointer.Int(page),
			PerPage: pointer.Int(perPage),
		}
		result, err := client.Collection(table).Documents().Search(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("typesense search %q: %w", table, err)
		}
		if result.Hits == nil || len(*result.Hits) == 0 {
			break
		}
		for _, hit := range *result.Hits {
			if hit.Document == nil {
				continue
			}
			doc := make(map[string]any, len(*hit.Document))
			for k, v := range *hit.Document {
				if k == SortableIDField {
					continue
				}
				doc[k] = v
			}
			rows = append(rows, doc)
		}
		if len(*result.Hits) < perPage {
			break
		}
		page++
	}
	return rows, nil
}
