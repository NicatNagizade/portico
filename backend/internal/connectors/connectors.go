package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portico/backend/internal/models"
)

type FieldType string

const (
	FieldTypeString      FieldType = "string"
	FieldTypeInt64       FieldType = "int64"
	FieldTypeFloat64     FieldType = "float64"
	FieldTypeBool        FieldType = "bool"
	FieldTypeObject      FieldType = "object"
	FieldTypeObjectArray FieldType = "object_array"
)

type ColumnSchema struct {
	Name       string
	Type       FieldType
	PrimaryKey bool
}

type TableSchema struct {
	Columns []ColumnSchema
}

type SourceReader interface {
	Open(ctx context.Context) error
	ListTables(ctx context.Context) ([]string, error)
	Schema(ctx context.Context, table string) (*TableSchema, error)
	Count(ctx context.Context, table string) (int64, error)
	ReadChunks(ctx context.Context, table string, chunkSize int, fn func([]map[string]any) error) error
	QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error)
	Close() error
}

type DestinationWriter interface {
	Open(ctx context.Context) error
	// Prepare sets up the destination. config is opaque JSON from sync_jobs.config;
	// each connector interprets keys it understands and ignores the rest / applies its own defaults.
	Prepare(ctx context.Context, name string, schema *TableSchema, config json.RawMessage) error
	WriteBatch(ctx context.Context, name string, docs []map[string]any) error
	Close() error
}

type SourceFactory func(conn *models.Connection) (SourceReader, error)
type DestinationFactory func(conn *models.Connection) (DestinationWriter, error)

type Registry struct {
	sources      map[string]SourceFactory
	destinations map[string]DestinationFactory
}

func NewRegistry() *Registry {
	return &Registry{
		sources:      make(map[string]SourceFactory),
		destinations: make(map[string]DestinationFactory),
	}
}

func (r *Registry) RegisterSource(typ string, factory SourceFactory) {
	r.sources[typ] = factory
}

func (r *Registry) RegisterDestination(typ string, factory DestinationFactory) {
	r.destinations[typ] = factory
}

func (r *Registry) NewSource(conn *models.Connection) (SourceReader, error) {
	factory, ok := r.sources[conn.Type]
	if !ok {
		return nil, fmt.Errorf("no source connector registered for type %q", conn.Type)
	}
	return factory(conn)
}

func (r *Registry) NewDestination(conn *models.Connection) (DestinationWriter, error) {
	factory, ok := r.destinations[conn.Type]
	if !ok {
		return nil, fmt.Errorf("no destination connector registered for type %q", conn.Type)
	}
	return factory(conn)
}

// Check opens the matching source or destination connector and closes it.
func (r *Registry) Check(ctx context.Context, conn *models.Connection) error {
	if _, ok := r.sources[conn.Type]; ok {
		src, err := r.NewSource(conn)
		if err != nil {
			return err
		}
		defer src.Close()
		return src.Open(ctx)
	}
	if _, ok := r.destinations[conn.Type]; ok {
		dst, err := r.NewDestination(conn)
		if err != nil {
			return err
		}
		defer dst.Close()
		return dst.Open(ctx)
	}
	return fmt.Errorf("no connector registered for type %q", conn.Type)
}

// EnsureID sets document id as a string from existing id, primary key, or row index.
func EnsureID(docs []map[string]any, schema *TableSchema, startIndex int64) {
	var pkCols []string
	for _, c := range schema.Columns {
		if c.PrimaryKey {
			pkCols = append(pkCols, c.Name)
		}
	}
	for i, doc := range docs {
		if id, ok := doc["id"]; ok && id != nil && fmt.Sprint(id) != "" {
			doc["id"] = fmt.Sprint(id)
			continue
		}
		if len(pkCols) > 0 {
			parts := make([]string, 0, len(pkCols))
			for _, pk := range pkCols {
				parts = append(parts, fmt.Sprint(doc[pk]))
			}
			doc["id"] = strings.Join(parts, "_")
			continue
		}
		doc["id"] = fmt.Sprintf("%d", startIndex+int64(i))
	}
}
