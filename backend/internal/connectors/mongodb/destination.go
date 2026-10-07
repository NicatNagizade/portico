package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Shorter than the driver's 30s default so unreachable hosts fail quickly on check/Open.
const connectTimeout = 2 * time.Second

// SortableIDField is the default numeric companion of document id for sorting.
// MongoDB "_id" is always a string document key (Portico "id"); override with primary_key_int.
const SortableIDField = connectors.DefaultSortableIDField

type Config struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password"`
	Database   string `json:"database"`
	AuthSource string `json:"auth_source"`
}

type Destination struct {
	cfg        Config
	client     *mongo.Client
	db         *mongo.Database
	sortableID string
}

func NewDestination(conn *models.Connection) (connectors.DestinationWriter, error) {
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
	}
	return &Destination{cfg: cfg, sortableID: SortableIDField}, nil
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

func connect(ctx context.Context, cfg Config) (*mongo.Client, *mongo.Database, error) {
	opts := options.Client().
		ApplyURI(BuildURI(cfg)).
		SetServerSelectionTimeout(connectTimeout).
		SetConnectTimeout(connectTimeout)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("mongodb connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("mongodb ping: %w", err)
	}
	return client, client.Database(cfg.Database), nil
}

func (d *Destination) Open(ctx context.Context) error {
	client, db, err := connect(ctx, d.cfg)
	if err != nil {
		return err
	}
	d.client = client
	d.db = db
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

func (d *Destination) Prepare(ctx context.Context, name string, schema *connectors.TableSchema, config json.RawMessage) error {
	if err := d.db.Collection(name).Drop(ctx); err != nil {
		return fmt.Errorf("drop mongodb collection %q: %w", name, err)
	}
	cfg, err := ParseCollectionConfig(config)
	if err != nil {
		return err
	}
	d.sortableID = cfg.SortableID()
	if !cfg.ApplySchema {
		return nil
	}
	validator, err := BuildValidator(schema, d.sortableID)
	if err != nil {
		return err
	}
	opts := options.CreateCollection().SetValidator(validator)
	if err := d.db.CreateCollection(ctx, name, opts); err != nil {
		return fmt.Errorf("create mongodb collection %q with schema: %w", name, err)
	}
	return nil
}

// ApplyJobConfig sets sync-job destination options (e.g. primary_key_int) for explore/Query.
func (d *Destination) ApplyJobConfig(raw json.RawMessage) error {
	cfg, err := ParseCollectionConfig(raw)
	if err != nil {
		return err
	}
	d.sortableID = cfg.SortableID()
	return nil
}

func (d *Destination) sortableIDField() string {
	if d != nil {
		return d.sortableID
	}
	return SortableIDField
}

func (d *Destination) WriteBatch(ctx context.Context, name string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}
	sortableID := d.sortableIDField()
	payload := make([]any, len(docs))
	for i, doc := range docs {
		payload[i] = DocForWrite(doc, sortableID)
	}
	_, err := d.db.Collection(name).InsertMany(ctx, payload)
	if err != nil {
		return fmt.Errorf("mongodb insert into %q: %w", name, err)
	}
	return nil
}

// DocForWrite copies doc, maps Portico "id" onto MongoDB "_id" (string),
// and sets the numeric companion from a numeric document id when possible.
func DocForWrite(doc map[string]any, sortableID string) map[string]any {
	out := make(map[string]any, len(doc)+2)
	for k, v := range doc {
		if k == "id" || (sortableID != "" && k == sortableID) {
			continue
		}
		out[k] = v
	}
	if id, ok := doc["id"]; ok && id != nil && fmt.Sprint(id) != "" {
		out["_id"] = fmt.Sprint(id)
		if sortableID != "" {
			if n, ok := docutil.ParseInt64(id); ok {
				out[sortableID] = n
			}
		}
	}
	return out
}

// DocFromRead copies a stored document and maps "_id" back to "id" for explore/UI.
// Nested BSON maps/arrays are normalized to map[string]any / []any.
// The numeric companion of id is an internal field — hidden from explore/UI.
func DocFromRead(doc map[string]any, sortableID string) map[string]any {
	out := make(map[string]any, len(doc))
	for k, v := range doc {
		if sortableID != "" && k == sortableID {
			continue
		}
		if k == "_id" {
			out["id"] = normalizeID(v)
			continue
		}
		out[k] = normalizeBSONValue(v)
	}
	return out
}

func normalizeID(v any) any {
	switch x := v.(type) {
	case bson.ObjectID:
		return x.Hex()
	default:
		return normalizeBSONValue(v)
	}
}

func normalizeBSONValue(v any) any {
	switch x := v.(type) {
	case bson.M:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = normalizeBSONValue(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = normalizeBSONValue(val)
		}
		return out
	case bson.A:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalizeBSONValue(val)
		}
		return out
	case bson.D:
		out := make(map[string]any, len(x))
		for _, e := range x {
			out[e.Key] = normalizeBSONValue(e.Value)
		}
		return out
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

	filter, err := BuildFilter(filters, d.sortableIDField())
	if err != nil {
		return nil, 0, err
	}

	coll := d.db.Collection(name)
	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("mongodb count %q: %w", name, err)
	}

	findOpts := options.Find().SetSkip(int64(offset)).SetLimit(int64(limit))
	if order != nil && strings.TrimSpace(order.Column) != "" {
		col := docutil.SortColumn(order.Column, d.sortableIDField())
		dir := 1
		if order.Desc {
			dir = -1
		}
		findOpts.SetSort(bson.D{{Key: col, Value: dir}})
	}

	cur, err := coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("mongodb find %q: %w", name, err)
	}
	defer cur.Close(ctx)

	rows, err := decodeDocs(cur, ctx, d.sortableIDField())
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func bsonMToMap(m bson.M) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func decodeDocs(cur *mongo.Cursor, ctx context.Context, sortableID string) ([]map[string]any, error) {
	rows := []map[string]any{}
	for cur.Next(ctx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			return nil, fmt.Errorf("mongodb decode: %w", err)
		}
		rows = append(rows, DocFromRead(bsonMToMap(raw), sortableID))
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("mongodb cursor: %w", err)
	}
	return rows, nil
}
