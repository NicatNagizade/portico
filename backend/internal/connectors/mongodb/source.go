package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Source struct {
	cfg    Config
	client *mongo.Client
	db     *mongo.Database
}

func NewSource(conn *models.Connection) (connectors.SourceReader, error) {
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
	}
	return &Source{cfg: cfg}, nil
}

func (s *Source) Open(ctx context.Context) error {
	client, db, err := connect(ctx, s.cfg)
	if err != nil {
		return err
	}
	s.client = client
	s.db = db
	return nil
}

func (s *Source) Close() error {
	if s.client == nil {
		return nil
	}
	err := s.client.Disconnect(context.Background())
	s.client = nil
	s.db = nil
	return err
}

func (s *Source) ListTables(ctx context.Context) ([]string, error) {
	names, err := s.db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("mongodb list collections: %w", err)
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}

func (s *Source) Schema(ctx context.Context, table string) (*connectors.TableSchema, error) {
	if schema, err := s.schemaFromCollectionValidator(ctx, table); err == nil && schema != nil && len(schema.Columns) > 0 {
		return schema, nil
	}
	docs, err := s.loadDocs(ctx, table, 50)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		// Empty collection: still expose id so sync jobs can target it.
		return &connectors.TableSchema{
			Columns: []connectors.ColumnSchema{
				{Name: "id", Type: connectors.FieldTypeString, PrimaryKey: true},
			},
		}, nil
	}
	return docutil.SchemaFromDocs(docs), nil
}

func (s *Source) schemaFromCollectionValidator(ctx context.Context, table string) (*connectors.TableSchema, error) {
	cur, err := s.db.ListCollections(ctx, bson.D{{Key: "name", Value: table}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	if !cur.Next(ctx) {
		return nil, nil
	}
	var info struct {
		Options bson.M `bson:"options"`
	}
	if err := cur.Decode(&info); err != nil {
		return nil, err
	}
	raw, ok := info.Options["validator"]
	if !ok || raw == nil {
		return nil, nil
	}
	validator, ok := asBSONMap(raw)
	if !ok {
		return nil, nil
	}
	schema := SchemaFromValidator(validator)
	if schema != nil {
		schema.Columns = docutil.NestDottedFields(schema.Columns)
	}
	return schema, nil
}

func (s *Source) Count(ctx context.Context, table string, filters []connectors.Filter) (int64, error) {
	filter, err := BuildFilter(filters, SortableIDField)
	if err != nil {
		return 0, err
	}
	n, err := s.db.Collection(table).CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("mongodb count %q: %w", table, err)
	}
	return n, nil
}

func (s *Source) ReadChunks(ctx context.Context, table string, chunkSize int, filters []connectors.Filter, fn func([]map[string]any) error) error {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	filter, err := BuildFilter(filters, SortableIDField)
	if err != nil {
		return err
	}
	cur, err := s.db.Collection(table).Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("mongodb find %q: %w", table, err)
	}
	defer cur.Close(ctx)

	batch := make([]map[string]any, 0, chunkSize)
	for cur.Next(ctx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			return fmt.Errorf("mongodb decode: %w", err)
		}
		batch = append(batch, DocFromRead(bsonMToMap(raw), SortableIDField))
		if len(batch) >= chunkSize {
			if err := fn(batch); err != nil {
				return err
			}
			batch = make([]map[string]any, 0, chunkSize)
		}
	}
	if err := cur.Err(); err != nil {
		return fmt.Errorf("mongodb cursor: %w", err)
	}
	if len(batch) > 0 {
		return fn(batch)
	}
	return nil
}

func (s *Source) Query(ctx context.Context, table string, columns []string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	filter, err := BuildFilter(filters, SortableIDField)
	if err != nil {
		return nil, err
	}

	findOpts := options.Find().SetSkip(int64(offset)).SetLimit(int64(limit))
	if order != nil && strings.TrimSpace(order.Column) != "" {
		col := docutil.SortColumn(order.Column, SortableIDField)
		dir := 1
		if order.Desc {
			dir = -1
		}
		findOpts.SetSort(bson.D{{Key: col, Value: dir}})
	}

	cur, err := s.db.Collection(table).Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("mongodb find %q: %w", table, err)
	}
	defer cur.Close(ctx)

	rows, err := decodeDocs(cur, ctx, SortableIDField)
	if err != nil {
		return nil, err
	}
	return docutil.SelectColumns(rows, columns), nil
}

func (s *Source) QueryRows(ctx context.Context, table string, columns []string, whereColumn string, whereValues []any) ([]map[string]any, error) {
	if len(whereValues) == 0 {
		return nil, nil
	}
	col := whereColumn
	if col == "id" {
		col = "_id"
	}
	filter := bson.D{{Key: col, Value: bson.D{{Key: "$in", Value: whereValues}}}}
	cur, err := s.db.Collection(table).Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("mongodb query rows %q: %w", table, err)
	}
	defer cur.Close(ctx)

	rows, err := decodeDocs(cur, ctx, SortableIDField)
	if err != nil {
		return nil, err
	}
	return docutil.SelectColumns(rows, columns), nil
}

// loadDocs reads documents from a collection. limit <= 0 means all.
func (s *Source) loadDocs(ctx context.Context, table string, limit int64) ([]map[string]any, error) {
	opts := options.Find()
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cur, err := s.db.Collection(table).Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("mongodb find %q: %w", table, err)
	}
	defer cur.Close(ctx)
	return decodeDocs(cur, ctx, SortableIDField)
}
