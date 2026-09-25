package typesense

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
	"github.com/typesense/typesense-go/v2/typesense"
	"github.com/typesense/typesense-go/v2/typesense/api"
	"github.com/typesense/typesense-go/v2/typesense/api/pointer"
)

// SortableIDField holds a numeric copy of document id for Typesense sort_by.
// Typesense reserves "id" as a string document key and strips it from the schema.
const SortableIDField = "id_int"

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
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
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
	coll, err := BuildCollectionSchema(name, schema, collCfg)
	if err != nil {
		return err
	}
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
// Document "id" is implicit (always string). A numeric id_int companion is added for sorting.
func BuildCollectionSchema(name string, schema *connectors.TableSchema, cfg *CollectionConfig) (*api.CollectionSchema, error) {
	fields := make([]api.Field, 0, len(schema.Columns)+1)
	for _, col := range schema.Columns {
		if col.Name == "id" {
			// Reserved document id — do not declare; Typesense would strip it anyway.
			continue
		}
		fields = append(fields, api.Field{
			Name:     col.Name,
			Type:     mapFieldType(col.Type),
			Optional: pointer.True(),
		})
	}
	fields = append(fields, api.Field{
		Name:     SortableIDField,
		Type:     "int64",
		Sort:     pointer.True(),
		Optional: pointer.True(),
	})

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
			sortField := resolveSortField(strings.TrimSpace(*cfg.DefaultSortingField))
			coll.DefaultSortingField = &sortField
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
	return coll, nil
}

func boolPtr(v bool) *bool {
	if v {
		return pointer.True()
	}
	return pointer.False()
}

// resolveSortField maps explore/config "id" to the sortable int companion.
func resolveSortField(col string) string {
	if col == "id" {
		return SortableIDField
	}
	return col
}

func (d *Destination) WriteBatch(ctx context.Context, name string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}
	payload := make([]interface{}, len(docs))
	for i, doc := range docs {
		payload[i] = docWithSortableID(doc)
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

// docWithSortableID copies doc and sets id_int from a numeric document id when possible.
func docWithSortableID(doc map[string]any) map[string]any {
	n, ok := toInt64(doc["id"])
	if !ok {
		return doc
	}
	out := make(map[string]any, len(doc)+1)
	for k, v := range doc {
		out[k] = v
	}
	out[SortableIDField] = n
	return out
}

func toInt64(v any) (int64, bool) {
	if v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		return int64(x), true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(x), true
	case float32:
		return int64(x), true
	case float64:
		return int64(x), true
	case json.Number:
		n, err := x.Int64()
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n, err == nil
	default:
		n, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(x)), 10, 64)
		return n, err == nil
	}
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
	// Non-schema fields (rare) fall back to in-memory sort for the current page.
	var localSort *connectors.Order
	if order != nil && strings.TrimSpace(order.Column) != "" {
		col := resolveSortField(strings.TrimSpace(order.Column))
		if sortableSchemaField(coll.Fields, col) {
			dir := "asc"
			if order.Desc {
				dir = "desc"
			}
			params.SortBy = pointer.String(col + ":" + dir)
		} else {
			localSort = &connectors.Order{Column: strings.TrimSpace(order.Column), Desc: order.Desc}
		}
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
			if k == SortableIDField {
				continue // internal sortable copy of id — hide from explore/UI
			}
			doc[k] = v
		}
		rows = append(rows, doc)
	}
	if localSort != nil {
		sortRowsByColumn(rows, localSort.Column, localSort.Desc)
	}
	return rows, total, nil
}

// sortableSchemaField reports whether Typesense can sort_by this field.
func sortableSchemaField(fields []api.Field, name string) bool {
	if name == "" || name == "id" {
		return false
	}
	for _, f := range fields {
		if f.Name != name {
			continue
		}
		if f.Sort != nil {
			return *f.Sort
		}
		switch f.Type {
		case "int32", "int64", "float":
			return true
		default:
			return false
		}
	}
	return false
}

func sortRowsByColumn(rows []map[string]any, col string, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		cmp := strings.Compare(fmt.Sprint(rows[i][col]), fmt.Sprint(rows[j][col]))
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

// searchableQueryBy picks Typesense string fields for q=* browse.
// The special document id field cannot be used in query_by.
func searchableQueryBy(fields []api.Field) string {
	var names []string
	for _, f := range fields {
		if f.Name == "" || f.Name == "id" || f.Name == SortableIDField {
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
