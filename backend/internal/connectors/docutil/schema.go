package docutil

import (
	"sort"
	"strings"

	"github.com/portico/backend/internal/connectors"
)

// SchemaFromDocs builds a TableSchema from sample documents.
// Nested objects / object-arrays populate ColumnSchema.Columns.
// id is marked primary key when present at the root.
func SchemaFromDocs(docs []map[string]any) *connectors.TableSchema {
	root := map[string]*colNode{}
	var order []string
	for _, doc := range docs {
		for k, v := range doc {
			mergeValue(root, &order, k, v)
		}
	}
	sort.Strings(order)
	if _, ok := root["id"]; ok {
		rest := make([]string, 0, len(order)-1)
		for _, k := range order {
			if k != "id" {
				rest = append(rest, k)
			}
		}
		order = append([]string{"id"}, rest...)
	}
	schema := &connectors.TableSchema{Columns: make([]connectors.ColumnSchema, 0, len(order))}
	for _, k := range order {
		schema.Columns = append(schema.Columns, root[k].toColumn(k, k == "id"))
	}
	// Fold any literal dotted keys (rare) under parents — Fields pickers use top-level only.
	schema.Columns = NestDottedFields(schema.Columns)
	return schema
}

type colNode struct {
	Type    connectors.FieldType
	Nested  map[string]*colNode
	Order   []string
}

func (n *colNode) toColumn(name string, pk bool) connectors.ColumnSchema {
	col := connectors.ColumnSchema{Name: name, Type: n.Type, PrimaryKey: pk}
	if len(n.Order) == 0 {
		return col
	}
	col.Columns = make([]connectors.ColumnSchema, 0, len(n.Order))
	for _, k := range n.Order {
		col.Columns = append(col.Columns, n.Nested[k].toColumn(k, false))
	}
	return col
}

func mergeValue(into map[string]*colNode, order *[]string, key string, v any) {
	ft := InferFieldType(v)
	node, ok := into[key]
	if !ok {
		node = &colNode{Type: ft, Nested: map[string]*colNode{}}
		into[key] = node
		*order = append(*order, key)
	} else if node.Type == connectors.FieldTypeString && ft != connectors.FieldTypeString {
		// Upgrade from null-inferred string when a richer type appears later.
		node.Type = ft
	}

	switch ft {
	case connectors.FieldTypeObject:
		if m, ok := asMap(v); ok {
			for nk, nv := range m {
				mergeValue(node.Nested, &node.Order, nk, nv)
			}
		}
	case connectors.FieldTypeObjectArray:
		for _, item := range asObjectSlice(v) {
			for nk, nv := range item {
				mergeValue(node.Nested, &node.Order, nk, nv)
			}
		}
	}
}

func asMap(v any) (map[string]any, bool) {
	switch x := v.(type) {
	case map[string]any:
		return x, true
	default:
		return nil, false
	}
}

func asObjectSlice(v any) []map[string]any {
	switch x := v.(type) {
	case []map[string]any:
		return x
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, item := range x {
			if m, ok := asMap(item); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

// FlattenColumnPaths expands nested Columns into dotted paths (e.g. posts.title).
// Parent object/array columns are included, then their nested fields.
func FlattenColumnPaths(cols []connectors.ColumnSchema) []connectors.ColumnSchema {
	var out []connectors.ColumnSchema
	var walk func(prefix string, cols []connectors.ColumnSchema, rootPK bool)
	walk = func(prefix string, cols []connectors.ColumnSchema, rootPK bool) {
		for _, c := range cols {
			if c.Name == "" {
				continue
			}
			path := c.Name
			if prefix != "" {
				path = prefix + "." + c.Name
			}
			out = append(out, connectors.ColumnSchema{
				Name:       path,
				Type:       c.Type,
				PrimaryKey: rootPK && c.PrimaryKey,
			})
			if len(c.Columns) > 0 {
				walk(path, c.Columns, false)
			}
		}
	}
	walk("", cols, true)
	return out
}

// NestDottedFields folds dotted field names (e.g. "posts.title") into
// ColumnSchema.Columns under their parent. Used when a store exposes nested
// paths as top-level names (Typesense enable_nested_fields). Root object /
// object[] fields stay; dotted siblings leave the root list.
// Explore Filters still reach nested paths via FlattenColumnPaths / UI flatten.
func NestDottedFields(cols []connectors.ColumnSchema) []connectors.ColumnSchema {
	byRoot := map[string]connectors.ColumnSchema{}
	var order []string
	nested := map[string][]connectors.ColumnSchema{}

	for _, c := range cols {
		if c.Name == "" {
			continue
		}
		name, rest, dotted := strings.Cut(c.Name, ".")
		if !dotted {
			if _, exists := byRoot[name]; !exists {
				order = append(order, name)
			}
			byRoot[name] = c
			continue
		}
		child := c
		child.Name = rest
		nested[name] = append(nested[name], child)
		if _, exists := byRoot[name]; !exists {
			order = append(order, name)
			byRoot[name] = connectors.ColumnSchema{Name: name, Type: connectors.FieldTypeObject}
		}
	}

	out := make([]connectors.ColumnSchema, 0, len(order))
	for _, name := range order {
		col := byRoot[name]
		if kids := nested[name]; len(kids) > 0 {
			col.Columns = NestDottedFields(kids)
			if col.Type != connectors.FieldTypeObject && col.Type != connectors.FieldTypeObjectArray {
				col.Type = connectors.FieldTypeObject
			}
		}
		out = append(out, col)
	}
	return out
}

// MergeNestedFrom fills empty Columns on object / object_array fields from sample.
// Existing nested Columns are left unchanged.
func MergeNestedFrom(base *connectors.TableSchema, sample *connectors.TableSchema) *connectors.TableSchema {
	if base == nil {
		return sample
	}
	if sample == nil {
		return base
	}
	byName := map[string]connectors.ColumnSchema{}
	for _, c := range sample.Columns {
		byName[c.Name] = c
	}
	out := &connectors.TableSchema{Columns: make([]connectors.ColumnSchema, len(base.Columns))}
	copy(out.Columns, base.Columns)
	for i, c := range out.Columns {
		if len(c.Columns) > 0 {
			continue
		}
		if c.Type != connectors.FieldTypeObject && c.Type != connectors.FieldTypeObjectArray {
			continue
		}
		if s, ok := byName[c.Name]; ok && len(s.Columns) > 0 {
			out.Columns[i].Columns = s.Columns
			if out.Columns[i].Type == "" {
				out.Columns[i].Type = s.Type
			}
		}
	}
	// Append sample-only columns (e.g. fields Typesense inferred but not declared).
	seen := map[string]struct{}{}
	for _, c := range out.Columns {
		seen[c.Name] = struct{}{}
	}
	for _, c := range sample.Columns {
		if _, ok := seen[c.Name]; ok {
			continue
		}
		out.Columns = append(out.Columns, c)
	}
	return out
}
