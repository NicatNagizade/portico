package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type Config struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password"`
	Database   string `json:"database"`
	AuthSource string `json:"auth_source"`
}

type Destination struct {
	cfg    Config
	client *mongo.Client
	db     *mongo.Database
}

func NewDestination(conn *models.Connection) (connectors.DestinationWriter, error) {
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
	}
	return &Destination{cfg: cfg}, nil
}

func ParseConfig(raw json.RawMessage) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse mongodb config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 27017
	}
	if cfg.Host == "" {
		return Config{}, fmt.Errorf("parse mongodb config: host is required")
	}
	if cfg.Database == "" {
		return Config{}, fmt.Errorf("parse mongodb config: database is required")
	}
	if cfg.AuthSource == "" {
		cfg.AuthSource = "admin"
	}
	return cfg, nil
}

// BuildURI builds a mongodb:// connection string from discrete config fields.
// Always sets directConnection=true so Docker replica-set hostnames are not followed.
func BuildURI(cfg Config) string {
	host := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	u := &url.URL{
		Scheme: "mongodb",
		Host:   host,
		Path:   "/" + cfg.Database,
	}
	if cfg.User != "" {
		u.User = url.UserPassword(cfg.User, cfg.Password)
	}
	q := url.Values{}
	if cfg.User != "" && cfg.AuthSource != "" {
		q.Set("authSource", cfg.AuthSource)
	}
	q.Set("directConnection", "true")
	u.RawQuery = q.Encode()
	return u.String()
}

func (d *Destination) Open(ctx context.Context) error {
	client, err := mongo.Connect(options.Client().ApplyURI(BuildURI(d.cfg)))
	if err != nil {
		return fmt.Errorf("mongodb connect: %w", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return fmt.Errorf("mongodb ping: %w", err)
	}
	d.client = client
	d.db = client.Database(d.cfg.Database)
	return nil
}

func (d *Destination) Close() error {
	if d.client == nil {
		return nil
	}
	err := d.client.Disconnect(context.Background())
	d.client = nil
	d.db = nil
	return err
}

func (d *Destination) Prepare(ctx context.Context, name string, _ *connectors.TableSchema, _ json.RawMessage) error {
	if err := d.db.Collection(name).Drop(ctx); err != nil {
		return fmt.Errorf("drop mongodb collection %q: %w", name, err)
	}
	return nil
}

func (d *Destination) WriteBatch(ctx context.Context, name string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}
	payload := make([]any, len(docs))
	for i, doc := range docs {
		payload[i] = DocForWrite(doc)
	}
	_, err := d.db.Collection(name).InsertMany(ctx, payload)
	if err != nil {
		return fmt.Errorf("mongodb insert into %q: %w", name, err)
	}
	return nil
}

// DocForWrite copies doc and maps Portico "id" onto MongoDB "_id".
func DocForWrite(doc map[string]any) map[string]any {
	out := make(map[string]any, len(doc)+1)
	for k, v := range doc {
		if k == "id" {
			continue
		}
		out[k] = v
	}
	if id, ok := doc["id"]; ok && id != nil && fmt.Sprint(id) != "" {
		out["_id"] = id
	}
	return out
}

// DocFromRead copies a stored document and maps "_id" back to "id" for explore/UI.
func DocFromRead(doc map[string]any) map[string]any {
	out := make(map[string]any, len(doc))
	for k, v := range doc {
		if k == "_id" {
			out["id"] = normalizeID(v)
			continue
		}
		out[k] = v
	}
	return out
}

func normalizeID(v any) any {
	switch x := v.(type) {
	case bson.ObjectID:
		return x.Hex()
	default:
		return v
	}
}

func (d *Destination) Query(ctx context.Context, name string, filters []connectors.Filter, limit, offset int, order *connectors.Order) ([]map[string]any, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	if len(filters) > 0 {
		docs, err := d.loadDocs(ctx, name)
		if err != nil {
			return nil, 0, err
		}
		docs = docutil.FilterRows(docs, filters)
		total := int64(len(docs))
		docutil.SortRows(docs, order)
		return docutil.PageRows(docs, limit, offset), total, nil
	}

	coll := d.db.Collection(name)
	total, err := coll.CountDocuments(ctx, bson.D{})
	if err != nil {
		return nil, 0, fmt.Errorf("mongodb count %q: %w", name, err)
	}

	findOpts := options.Find().SetSkip(int64(offset)).SetLimit(int64(limit))
	if order != nil && strings.TrimSpace(order.Column) != "" {
		col := strings.TrimSpace(order.Column)
		if col == "id" {
			col = "_id"
		}
		dir := 1
		if order.Desc {
			dir = -1
		}
		findOpts.SetSort(bson.D{{Key: col, Value: dir}})
	}

	cur, err := coll.Find(ctx, bson.D{}, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("mongodb find %q: %w", name, err)
	}
	defer cur.Close(ctx)

	rows := []map[string]any{}
	for cur.Next(ctx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			return nil, 0, fmt.Errorf("mongodb decode: %w", err)
		}
		rows = append(rows, DocFromRead(bsonMToMap(raw)))
	}
	if err := cur.Err(); err != nil {
		return nil, 0, fmt.Errorf("mongodb cursor: %w", err)
	}
	return rows, total, nil
}

func (d *Destination) loadDocs(ctx context.Context, name string) ([]map[string]any, error) {
	cur, err := d.db.Collection(name).Find(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("mongodb find %q: %w", name, err)
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

func bsonMToMap(m bson.M) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
