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

type Destination struct {
	cfg    Config
	client *goredis.Client
}

func NewDestination(conn *models.Connection) (connectors.DestinationWriter, error) {
	cfg, err := ParseConfig(json.RawMessage(conn.Config))
	if err != nil {
		return nil, err
	}
	return &Destination{cfg: cfg}, nil
}

func (d *Destination) Open(ctx context.Context) error {
	client := goredis.NewClient(&goredis.Options{
		Addr:     d.cfg.Addr(),
		Password: d.cfg.Password,
		DB:       d.cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return fmt.Errorf("redis ping: %w", err)
	}
	d.client = client
	return nil
}

func (d *Destination) Close() error {
	if d.client == nil {
		return nil
	}
	err := d.client.Close()
	d.client = nil
	return err
}

func (d *Destination) Prepare(ctx context.Context, name string, _ *connectors.TableSchema, _ json.RawMessage) error {
	if err := deleteKeysByPattern(ctx, d.client, d.cfg.KeyPattern(name)); err != nil {
		return fmt.Errorf("redis prepare delete %q: %w", name, err)
	}
	if err := d.client.SAdd(ctx, TablesCatalogKey, name).Err(); err != nil {
		return fmt.Errorf("redis catalog add %q: %w", name, err)
	}
	return nil
}

func (d *Destination) WriteBatch(ctx context.Context, name string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}
	pipe := d.client.Pipeline()
	for _, doc := range docs {
		id := fmt.Sprint(doc["id"])
		if id == "" || id == "<nil>" {
			return fmt.Errorf("redis write into %q: document missing id", name)
		}
		b, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("redis marshal: %w", err)
		}
		pipe.Set(ctx, d.cfg.DocKey(name, id), b, 0)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis write into %q: %w", name, err)
	}
	return nil
}

func (d *Destination) Query(ctx context.Context, name string, limit, offset int, order *connectors.Order) ([]map[string]any, int64, error) {
	docs, err := loadTableDocs(ctx, d.client, d.cfg, name)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(docs))
	docutil.SortRows(docs, order)
	return docutil.PageRows(docs, limit, offset), total, nil
}

func deleteKeysByPattern(ctx context.Context, client *goredis.Client, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func loadTableDocs(ctx context.Context, client *goredis.Client, cfg Config, table string) ([]map[string]any, error) {
	var cursor uint64
	var keys []string
	pattern := cfg.KeyPattern(table)
	for {
		batch, next, err := client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan %q: %w", table, err)
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return []map[string]any{}, nil
	}
	vals, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis mget %q: %w", table, err)
	}
	docs := make([]map[string]any, 0, len(vals))
	for i, v := range vals {
		if v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(s), &doc); err != nil {
			return nil, fmt.Errorf("redis unmarshal %q: %w", keys[i], err)
		}
		if doc == nil {
			doc = map[string]any{}
		}
		if _, hasID := doc["id"]; !hasID {
			doc["id"] = cfg.IDFromKey(table, keys[i])
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
