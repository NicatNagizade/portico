package typesense

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
	"github.com/typesense/typesense-go/v2/typesense"
	"github.com/typesense/typesense-go/v2/typesense/api"
	"github.com/typesense/typesense-go/v2/typesense/api/pointer"
)

type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	APIKey   string `json:"api_key"`
	Protocol string `json:"protocol"`
}

// CollectionConfig is Typesense-specific options from sync_jobs.config.
// Omitted fields keep connector defaults.
type CollectionConfig struct {
	DefaultSortingField *string  `json:"default_sorting_field,omitempty"`
	EnableNestedFields  *bool    `json:"enable_nested_fields,omitempty"`
	SymbolsToIndex      []string `json:"symbols_to_index,omitempty"`
	TokenSeparators     []string `json:"token_separators,omitempty"`
}

type Destination struct {
	cfg    Config
	client *typesense.Client
}

func NewDestination(conn *models.Connection) (connectors.DestinationWriter, error) {
	var cfg Config
	if err := json.Unmarshal(conn.Config, &cfg); err != nil {
		return nil, fmt.Errorf("parse typesense config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 8108
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "http"
	}
	return &Destination{cfg: cfg}, nil
}

func (d *Destination) Open(ctx context.Context) error {
	node := fmt.Sprintf("%s://%s:%d", d.cfg.Protocol, d.cfg.Host, d.cfg.Port)
	d.client = typesense.NewClient(
		typesense.WithNodes([]string{node}),
		typesense.WithAPIKey(d.cfg.APIKey),
	)
	if _, err := d.client.Health(ctx, 5*time.Second); err != nil {
		return fmt.Errorf("typesense health check: %w", err)
	}
	return nil
}

func (d *Destination) Close() error { return nil }

func (d *Destination) Prepare(ctx context.Context, name string, schema *connectors.TableSchema, config json.RawMessage) error {
	_, _ = d.client.Collection(name).Delete(ctx)

	collCfg, err := ParseCollectionConfig(config)
	if err != nil {
		return err
	}
	coll := BuildCollectionSchema(name, schema, collCfg)
	_, err = d.client.Collections().Create(ctx, coll)
	if err != nil {
		return fmt.Errorf("create typesense collection %q: %w", name, err)
	}
	return nil
}

// ParseCollectionConfig decodes Typesense collection options from sync job config JSON.
func ParseCollectionConfig(raw json.RawMessage) (*CollectionConfig, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var cfg CollectionConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse typesense collection config: %w", err)
	}
	return &cfg, nil
}

// BuildCollectionSchema maps a Portico table schema to a Typesense collection schema.
func BuildCollectionSchema(name string, schema *connectors.TableSchema, cfg *CollectionConfig) *api.CollectionSchema {
	fields := make([]api.Field, 0, len(schema.Columns)+1)
	hasID := false
	for _, col := range schema.Columns {
		fieldType := mapFieldType(col.Type)
		// Typesense document id must always be a string.
		if col.Name == "id" {
			hasID = true
			fieldType = "string"
		}
		fields = append(fields, api.Field{
			Name:     col.Name,
			Type:     fieldType,
			Optional: pointer.True(),
		})
	}
	if !hasID {
		fields = append(fields, api.Field{Name: "id", Type: "string"})
	}

	enableNested := true
	if cfg != nil && cfg.EnableNestedFields != nil {
		enableNested = *cfg.EnableNestedFields
	}

	coll := &api.CollectionSchema{
		Name:               name,
		Fields:             fields,
		EnableNestedFields: boolPtr(enableNested),
	}
	if cfg != nil {
		if cfg.DefaultSortingField != nil && *cfg.DefaultSortingField != "" {
			coll.DefaultSortingField = cfg.DefaultSortingField
		}
		if len(cfg.SymbolsToIndex) > 0 {
			symbols := append([]string(nil), cfg.SymbolsToIndex...)
			coll.SymbolsToIndex = &symbols
		}
		if len(cfg.TokenSeparators) > 0 {
			seps := append([]string(nil), cfg.TokenSeparators...)
			coll.TokenSeparators = &seps
		}
	}
	return coll
}

func boolPtr(v bool) *bool {
	if v {
		return pointer.True()
	}
	return pointer.False()
}

func (d *Destination) WriteBatch(ctx context.Context, name string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}
	payload := make([]interface{}, len(docs))
	for i, doc := range docs {
		payload[i] = doc
	}
	results, err := d.client.Collection(name).Documents().Import(ctx, payload, &api.ImportDocumentsParams{
		Action:    pointer.String("create"),
		BatchSize: pointer.Int(len(docs)),
	})
	if err != nil {
		return fmt.Errorf("typesense import: %w", err)
	}
	for _, r := range results {
		if !r.Success {
			msg := r.Error
			if msg == "" {
				msg = "unknown error"
			}
			return fmt.Errorf("typesense import document failed: %s", msg)
		}
	}
	return nil
}

// Query reads documents from a Typesense collection (paginated). Used by explore.
func (d *Destination) Query(ctx context.Context, name string, limit, offset int, order *connectors.Order) ([]map[string]any, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	coll, err := d.client.Collection(name).Retrieve(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("typesense retrieve collection %q: %w", name, err)
	}
	queryBy := searchableQueryBy(coll.Fields)
	if queryBy == "" {
		return nil, 0, fmt.Errorf("typesense collection %q has no searchable string fields to browse", name)
	}

	params := &api.SearchCollectionParams{
		Q:       pointer.String("*"),
		QueryBy: pointer.String(queryBy),
		Page:    pointer.Int(offset/limit + 1),
		PerPage: pointer.Int(limit),
	}
	if order != nil && strings.TrimSpace(order.Column) != "" {
		dir := "asc"
		if order.Desc {
			dir = "desc"
		}
		params.SortBy = pointer.String(strings.TrimSpace(order.Column) + ":" + dir)
	}

	result, err := d.client.Collection(name).Documents().Search(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("typesense search %q: %w", name, err)
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
			doc[k] = v
		}
		rows = append(rows, doc)
	}
	return rows, total, nil
}

// searchableQueryBy picks Typesense string fields for q=* browse.
// The special document id field cannot be used in query_by.
func searchableQueryBy(fields []api.Field) string {
	var names []string
	for _, f := range fields {
		if f.Name == "" || f.Name == "id" {
			continue
		}
		if f.Index != nil && !*f.Index {
			continue
		}
		switch f.Type {
		case "string", "string[]":
			names = append(names, f.Name)
		}
	}
	return strings.Join(names, ",")
}

func mapFieldType(t connectors.FieldType) string {
	switch t {
	case connectors.FieldTypeInt64:
		return "int64"
	case connectors.FieldTypeFloat64:
		return "float"
	case connectors.FieldTypeBool:
		return "bool"
	case connectors.FieldTypeObject:
		return "object"
	case connectors.FieldTypeObjectArray:
		return "object[]"
	default:
		return "string"
	}
}
