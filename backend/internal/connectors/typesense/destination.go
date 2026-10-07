package typesense

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	"github.com/typesense/typesense-go/v2/typesense"
	"github.com/typesense/typesense-go/v2/typesense/api"
	"github.com/typesense/typesense-go/v2/typesense/api/pointer"
)

// SortableIDField is the default numeric companion of document id for Typesense sort_by.
// Typesense reserves "id" as a string document key and strips it from the schema.
// Override per job with config primary_key_int.
const SortableIDField = connectors.DefaultSortableIDField

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
	PrimaryKeyInt       string   `json:"primary_key_int,omitempty"` // legacy; prefer primary_key.int

	pk connectors.PrimaryKeyConfig
}

// SortableID returns the numeric companion field name, or "" when disabled.
func (c *CollectionConfig) SortableID() string {
	if c == nil {
		return SortableIDField
	}
	if c.pk.Configured {
		return c.pk.Int
	}
	if c.PrimaryKeyInt != "" {
		return c.PrimaryKeyInt
	}
	if c.pk.Int != "" {
		return c.pk.Int
	}
	return SortableIDField
}

type Destination struct {
	cfg        Config
	client     *typesense.Client
	sortableID string // from job config; empty = no companion
}

func NewDestination(conn *models.Connection) (connectors.DestinationWriter, error) {
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
	}
	return &Destination{cfg: cfg, sortableID: SortableIDField}, nil
}

func (d *Destination) Open(ctx context.Context) error {
	client, err := openClient(ctx, d.cfg)
	if err != nil {
		return err
	}
	d.client = client
	return nil
}

func (d *Destination) Close() error { return nil }

func (d *Destination) sortableIDField() string {
	if d != nil {
		return d.sortableID
	}
	return SortableIDField
}

// ApplyJobConfig sets sync-job destination options for explore/Query.
func (d *Destination) ApplyJobConfig(raw json.RawMessage) error {
	collCfg, err := ParseCollectionConfig(raw)
	if err != nil {
		return err
	}
	d.sortableID = collCfg.SortableID()
	return nil
}

func (d *Destination) Prepare(ctx context.Context, name string, schema *connectors.TableSchema, config json.RawMessage) error {
	_, _ = d.client.Collection(name).Delete(ctx)

	collCfg, err := ParseCollectionConfig(config)
	if err != nil {
		return err
	}
	d.sortableID = collCfg.SortableID()
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
		return &CollectionConfig{pk: connectors.PrimaryKeyConfig{Int: SortableIDField}}, nil
	}
	var cfg CollectionConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse typesense collection config: %w", err)
	}
	cfg.PrimaryKeyInt = strings.TrimSpace(cfg.PrimaryKeyInt)
	pk, err := connectors.ParsePrimaryKeyConfig(raw)
	if err != nil {
		return nil, err
	}
	cfg.pk = pk
	return &cfg, nil
}

// BuildCollectionSchema maps a Portico table schema to a Typesense collection schema.
// Document "id" is implicit (always string). Optional numeric companion is added when configured.
func BuildCollectionSchema(name string, schema *connectors.TableSchema, cfg *CollectionConfig) (*api.CollectionSchema, error) {
	sortableID := ""
	if cfg != nil {
		sortableID = cfg.SortableID()
	} else {
		sortableID = SortableIDField
	}
	fields := make([]api.Field, 0, len(schema.Columns)+1)
	for _, col := range schema.Columns {
		if col.Name == "id" || (sortableID != "" && col.Name == sortableID) {
			continue
		}
		fields = append(fields, api.Field{
			Name:     col.Name,
			Type:     mapFieldType(col.Type),
			Optional: pointer.True(),
		})
	}
	if sortableID != "" {
		fields = append(fields, api.Field{
			Name:     sortableID,
			Type:     "int64",
			Sort:     pointer.True(),
			Optional: pointer.True(),
		})
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
			sortField := strings.TrimSpace(*cfg.DefaultSortingField)
			if sortableID != "" {
				sortField = docutil.SortColumn(sortField, sortableID)
			}
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

func (d *Destination) WriteBatch(ctx context.Context, name string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}
	sortableID := d.sortableIDField()
	payload := make([]interface{}, len(docs))
	for i, doc := range docs {
		payload[i] = docWithSortableID(doc, sortableID)
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

// docWithSortableID copies doc and sets the numeric companion from document id when possible.
func docWithSortableID(doc map[string]any, sortableID string) map[string]any {
	if sortableID == "" {
		return doc
	}
	n, ok := docutil.ParseInt64(doc["id"])
	if !ok {
		return doc
	}
	out := make(map[string]any, len(doc)+1)
	for k, v := range doc {
		out[k] = v
	}
	out[sortableID] = n
	return out
}

// Query reads documents from a Typesense collection (paginated). Used by explore.
func (d *Destination) Query(ctx context.Context, name string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Operators Typesense cannot express (like / is_null / …) fall back to a full scan.
	filterBy, ok := BuildFilterBy(filters, d.sortableIDField())
	if len(filters) > 0 && !ok {
		docs, err := loadAll(ctx, d.client, name)
		if err != nil {
			return nil, 0, err
		}
		docs = docutil.FilterRows(docs, filters)
		total := int64(len(docs))
		docutil.SortRows(docs, order)
		return docutil.PageRows(docs, limit, offset), total, nil
	}

	coll, err := d.client.Collection(name).Retrieve(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("typesense retrieve collection %q: %w", name, err)
	}
	queryBy := searchableQueryBy(coll.Fields, d.sortableIDField())
	if queryBy == "" {
		return nil, 0, fmt.Errorf("typesense collection %q has no searchable string fields to browse", name)
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
	// Non-schema fields (rare) fall back to in-memory sort for the current page.
	var localSort *connectors.Order
	if order != nil && strings.TrimSpace(order.Column) != "" {
		col := docutil.SortColumn(strings.TrimSpace(order.Column), d.sortableIDField())
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
	sortableID := d.sortableIDField()
	for _, hit := range *result.Hits {
		if hit.Document == nil {
			continue
		}
		doc := make(map[string]any, len(*hit.Document))
		for k, v := range *hit.Document {
			if k == sortableID {
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
func searchableQueryBy(fields []api.Field, sortableID string) string {
	if sortableID == "" {
		sortableID = SortableIDField
	}
	var names []string
	for _, f := range fields {
		if f.Name == "" || f.Name == "id" || f.Name == sortableID {
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
