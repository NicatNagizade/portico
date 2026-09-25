package mongodb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
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
	client, err := mongo.Connect(options.Client().ApplyURI(BuildURI(s.cfg)))
	if err != nil {
		return fmt.Errorf("mongodb connect: %w", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return fmt.Errorf("mongodb ping: %w", err)
	}
	s.client = client
	s.db = client.Database(s.cfg.Database)
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

func (s *Source) Count(ctx context.Context, table string, filters []connectors.Filter) (int64, error) {
	if len(filters) == 0 {
		n, err := s.db.Collection(table).CountDocuments(ctx, bson.D{})
		if err != nil {
			return 0, fmt.Errorf("mongodb count %q: %w", table, err)
		}
		return n, nil
	}
	docs, err := s.loadDocs(ctx, table, 0)
	if err != nil {
		return 0, err
	}
	return int64(len(docutil.FilterRows(docs, filters))), nil
}

func (s *Source) ReadChunks(ctx context.Context, table string, chunkSize int, filters []connectors.Filter, fn func([]map[string]any) error) error {
	docs, err := s.loadDocs(ctx, table, 0)
	if err != nil {
		return err
	}
	return docutil.ReadChunksInMemory(docs, chunkSize, filters, fn)
}

func (s *Source) Query(ctx context.Context, table string, columns []string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, error) {
	docs, err := s.loadDocs(ctx, table, 0)
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

	rows := []map[string]any{}
	for cur.Next(ctx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			return nil, fmt.Errorf("mongodb decode: %w", err)
		}
		rows = append(rows, DocFromRead(bsonMToMap(raw)))
	}
	if err := cur.Err(); err != nil {
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

	rows := []map[string]any{}
	for cur.Next(ctx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			return nil, fmt.Errorf("mongodb decode: %w", err)
		}
		rows = append(rows, DocFromRead(bsonMToMap(raw)))
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("mongodb cursor: %w", err)
	}
	return rows, nil
}
