package mongodb

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// CollectionConfig is MongoDB-specific options from sync_jobs.config.
// Omitted fields keep connector defaults (no schema validator).
type CollectionConfig struct {
	ApplySchema bool `json:"apply_schema"`
}

// ParseCollectionConfig decodes MongoDB collection options from sync job config JSON.
func ParseCollectionConfig(raw json.RawMessage) (*CollectionConfig, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var cfg CollectionConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse mongodb collection config: %w", err)
	}
	return &cfg, nil
}

// BuildValidator maps a Portico table schema to a MongoDB $jsonSchema validator.
// Portico "id" becomes MongoDB "_id" (always string — EnsureID stringifies before write).
// Nested Columns (relations) become object properties or array items.
// Null is allowed on every property. SQL timestamps arrive as time.Time → BSON date.
func BuildValidator(schema *connectors.TableSchema) (bson.M, error) {
	if schema == nil || len(schema.Columns) == 0 {
		return nil, fmt.Errorf("mongodb apply_schema: table schema is required")
	}
	props := propertiesFromColumns(schema.Columns, true)
	if len(props) == 0 {
		return nil, fmt.Errorf("mongodb apply_schema: table schema has no columns")
	}
	return bson.M{
		"$jsonSchema": bson.M{
			"bsonType":   "object",
			"properties": props,
		},
	}, nil
}

// SchemaFromValidator maps a MongoDB $jsonSchema validator back to a Portico table schema.
// Returns nil when the validator is missing or has no properties.
func SchemaFromValidator(validator bson.M) *connectors.TableSchema {
	if len(validator) == 0 {
		return nil
	}
	js, ok := asBSONMap(validator["$jsonSchema"])
	if !ok {
		js, ok = asBSONMap(validator)
		if !ok {
			return nil
		}
	}
	props, ok := asBSONMap(js["properties"])
	if !ok || len(props) == 0 {
		return nil
	}
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)
	// Prefer id/_id first.
	ordered := make([]string, 0, len(names))
	for _, name := range names {
		if name == "_id" || name == "id" {
			ordered = append(ordered, name)
		}
	}
	for _, name := range names {
		if name == "_id" || name == "id" {
			continue
		}
		ordered = append(ordered, name)
	}
	schema := &connectors.TableSchema{Columns: make([]connectors.ColumnSchema, 0, len(ordered))}
	for _, name := range ordered {
		prop, ok := asBSONMap(props[name])
		if !ok {
			continue
		}
		colName := name
		pk := false
		if name == "_id" {
			colName = "id"
			pk = true
		}
		schema.Columns = append(schema.Columns, columnFromValidator(colName, prop, pk))
	}
	if len(schema.Columns) == 0 {
		return nil
	}
	return schema
}

func columnFromValidator(name string, prop bson.M, pk bool) connectors.ColumnSchema {
	col := connectors.ColumnSchema{
		Name:       name,
		Type:       bsonTypesToFieldType(prop["bsonType"]),
		PrimaryKey: pk,
	}
	if nested, ok := asBSONMap(prop["properties"]); ok && len(nested) > 0 {
		col.Type = connectors.FieldTypeObject
		col.Columns = propertiesToColumns(nested)
		return col
	}
	if items, ok := asBSONMap(prop["items"]); ok {
		col.Type = connectors.FieldTypeObjectArray
		if nested, ok := asBSONMap(items["properties"]); ok {
			col.Columns = propertiesToColumns(nested)
		}
	}
	return col
}

func propertiesToColumns(props bson.M) []connectors.ColumnSchema {
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]connectors.ColumnSchema, 0, len(names))
	for _, name := range names {
		prop, ok := asBSONMap(props[name])
		if !ok {
			continue
		}
		out = append(out, columnFromValidator(name, prop, false))
	}
	return out
}

func asBSONMap(v any) (bson.M, bool) {
	switch x := v.(type) {
	case bson.M:
		return x, true
	case map[string]any:
		out := bson.M{}
		for k, val := range x {
			out[k] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func bsonTypesToFieldType(v any) connectors.FieldType {
	types := bsonTypeList(v)
	has := func(want string) bool {
		for _, t := range types {
			if t == want {
				return true
			}
		}
		return false
	}
	switch {
	case has("array"):
		return connectors.FieldTypeObjectArray
	case has("object"):
		return connectors.FieldTypeObject
	case has("bool"):
		return connectors.FieldTypeBool
	case has("double"):
		return connectors.FieldTypeFloat64
	case has("int"), has("long"):
		return connectors.FieldTypeInt64
	default:
		return connectors.FieldTypeString
	}
}

func bsonTypeList(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" || x == "null" {
			return nil
		}
		return []string{x}
	case []string:
		out := make([]string, 0, len(x))
		for _, t := range x {
			if t != "" && t != "null" {
				out = append(out, t)
			}
		}
		return out
	case bson.A:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" && s != "null" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" && s != "null" {
				out = append(out, s)
			}
		}
		return out
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "null" || s == "<nil>" {
			return nil
		}
		return []string{s}
	}
}

// propertiesFromColumns builds JSON Schema properties.
// When root is true, Portico "id" is emitted as MongoDB "_id" (string).
func propertiesFromColumns(cols []connectors.ColumnSchema, root bool) bson.M {
	props := bson.M{}
	for _, col := range cols {
		if col.Name == "" {
			continue
		}
		if root && col.Name == "id" {
			props["_id"] = bson.M{"bsonType": []string{"string", "null"}}
			continue
		}
		props[col.Name] = columnValidator(col)
	}
	return props
}

func columnValidator(col connectors.ColumnSchema) bson.M {
	switch col.Type {
	case connectors.FieldTypeObject:
		m := bson.M{"bsonType": []string{"object", "null"}}
		if nested := propertiesFromColumns(col.Columns, false); len(nested) > 0 {
			m["properties"] = nested
		}
		return m
	case connectors.FieldTypeObjectArray:
		item := bson.M{"bsonType": "object"}
		if nested := propertiesFromColumns(col.Columns, false); len(nested) > 0 {
			item["properties"] = nested
		}
		return bson.M{
			"bsonType": []string{"array", "null"},
			"items":    item,
		}
	default:
		return bson.M{"bsonType": mapBSONTypes(col.Type)}
	}
}

func mapBSONTypes(t connectors.FieldType) []string {
	switch t {
	case connectors.FieldTypeInt64:
		return []string{"int", "long", "null"}
	case connectors.FieldTypeFloat64:
		return []string{"double", "int", "long", "null"}
	case connectors.FieldTypeBool:
		return []string{"bool", "null"}
	case connectors.FieldTypeObject:
		return []string{"object", "null"}
	case connectors.FieldTypeObjectArray:
		return []string{"array", "null"}
	default:
		// string covers text/uuid; date covers SQL timestamp/timestamptz values.
		return []string{"string", "date", "null"}
	}
}
