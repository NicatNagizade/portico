package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	goredis "github.com/redis/go-redis/v9"
)

const (
	scanCount    = 1000
	mgetChunk    = 500
	schemaSample = 50
)

func scanKeys(ctx context.Context, client *goredis.Client, pattern string, limit int) ([]string, error) {
	var cursor uint64
	var keys []string
	for {
		batch, next, err := client.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		if limit > 0 && len(keys) >= limit {
			return keys[:limit], nil
		}
		cursor = next
		if cursor == 0 {
			return keys, nil
		}
	}
}

func mgetDocs(ctx context.Context, client *goredis.Client, cfg Config, table string, keys []string) ([]map[string]any, error) {
	if len(keys) == 0 {
		return []map[string]any{}, nil
	}
	docs := make([]map[string]any, 0, len(keys))
	for i := 0; i < len(keys); i += mgetChunk {
		end := i + mgetChunk
		if end > len(keys) {
			end = len(keys)
		}
		chunk := keys[i:end]
		vals, err := client.MGet(ctx, chunk...).Result()
		if err != nil {
			return nil, fmt.Errorf("redis mget %q: %w", table, err)
		}
		for j, v := range vals {
			if v == nil {
				continue
			}
			s, ok := v.(string)
			if !ok {
				continue
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(s), &doc); err != nil {
				return nil, fmt.Errorf("redis unmarshal %q: %w", chunk[j], err)
			}
			if doc == nil {
				doc = map[string]any{}
			}
			if _, hasID := doc["id"]; !hasID {
				doc["id"] = cfg.IDFromKey(table, chunk[j])
			}
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

func loadTableDocs(ctx context.Context, client *goredis.Client, cfg Config, table string) ([]map[string]any, error) {
	keys, err := scanKeys(ctx, client, cfg.KeyPattern(table), 0)
	if err != nil {
		return nil, fmt.Errorf("redis scan %q: %w", table, err)
	}
	sort.Strings(keys)
	return mgetDocs(ctx, client, cfg, table, keys)
}

func loadTableDocsSample(ctx context.Context, client *goredis.Client, cfg Config, table string, n int) ([]map[string]any, error) {
	if n <= 0 {
		n = schemaSample
	}
	keys, err := scanKeys(ctx, client, cfg.KeyPattern(table), n)
	if err != nil {
		return nil, fmt.Errorf("redis scan %q: %w", table, err)
	}
	return mgetDocs(ctx, client, cfg, table, keys)
}

func countKeys(ctx context.Context, client *goredis.Client, pattern string) (int64, error) {
	var cursor uint64
	var n int64
	for {
		batch, next, err := client.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return 0, err
		}
		n += int64(len(batch))
		cursor = next
		if cursor == 0 {
			return n, nil
		}
	}
}

// idLookupKeys returns Redis keys when filters can be answered by id key lookup alone.
func idLookupKeys(cfg Config, table string, filters []connectors.Filter) ([]string, bool) {
	if len(filters) != 1 {
		return nil, false
	}
	f := filters[0]
	if strings.TrimSpace(f.Column) != "id" {
		return nil, false
	}
	switch f.Operator {
	case models.RuleOperatorEq:
		v := strings.TrimSpace(f.Value)
		if v == "" {
			return nil, false
		}
		return []string{cfg.DocKey(table, v)}, true
	case models.RuleOperatorIn:
		parts := strings.Split(f.Value, ",")
		keys := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			keys = append(keys, cfg.DocKey(table, p))
		}
		if len(keys) == 0 {
			return nil, false
		}
		return keys, true
	default:
		return nil, false
	}
}

func orderByIDOnly(order *connectors.Order) bool {
	return order == nil || strings.TrimSpace(order.Column) == "" || strings.TrimSpace(order.Column) == "id"
}

func reverseStrings(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// queryDocs loads, filters, sorts, and pages Redis documents with cheaper paths when possible.
func queryDocs(
	ctx context.Context,
	client *goredis.Client,
	cfg Config,
	table string,
	filters []connectors.Filter,
	limit, offset int,
	order *connectors.Order,
) ([]map[string]any, int64, error) {
	if keys, ok := idLookupKeys(cfg, table, filters); ok {
		docs, err := mgetDocs(ctx, client, cfg, table, keys)
		if err != nil {
			return nil, 0, err
		}
		docutil.SortRows(docs, order)
		total := int64(len(docs))
		return docutil.PageRows(docs, limit, offset), total, nil
	}

	// Unfiltered + id/default order: scan keys only, MGET the page.
	if len(filters) == 0 && orderByIDOnly(order) {
		keys, err := scanKeys(ctx, client, cfg.KeyPattern(table), 0)
		if err != nil {
			return nil, 0, fmt.Errorf("redis scan %q: %w", table, err)
		}
		sort.Strings(keys)
		if order != nil && order.Desc {
			reverseStrings(keys)
		}
		total := int64(len(keys))
		docs, err := mgetDocs(ctx, client, cfg, table, pageKeys(keys, limit, offset))
		if err != nil {
			return nil, 0, err
		}
		return docs, total, nil
	}

	docs, err := loadTableDocs(ctx, client, cfg, table)
	if err != nil {
		return nil, 0, err
	}
	docs = docutil.FilterRows(docs, filters)
	total := int64(len(docs))
	docutil.SortRows(docs, order)
	return docutil.PageRows(docs, limit, offset), total, nil
}

func pageKeys(keys []string, limit, offset int) []string {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(keys) {
		return nil
	}
	end := offset + limit
	if end > len(keys) {
		end = len(keys)
	}
	return keys[offset:end]
}
