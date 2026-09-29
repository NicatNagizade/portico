package connectors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// DefaultSortableIDField is the default numeric companion of document "id".
const DefaultSortableIDField = "id_int"

// PrimaryKeyConfig maps optional sync_jobs.config primary_key settings.
// When Configured is false, connectors keep defaults (document id = "id", int companion = id_int).
type PrimaryKeyConfig struct {
	Source      []string // source column(s); joined with "_" into Destination
	Destination string   // destination document/column key; default "id"
	Int         string   // numeric companion field; empty = do not create one
	Configured  bool     // true when primary_key was present in job config
}

// ParsePrimaryKeyConfig reads primary_key (+ optional legacy primary_key_int) from job config JSON.
//
// Accepted primary_key shapes:
//   - omitted → Configured=false (defaults)
//   - "uid" or ["user_id","post_id"]
//   - {"source":"uid"|"…", "destination":"id", "int":"id_int"}
//
// int is optional. When primary_key is configured and int is omitted, no companion field is created.
// When primary_key is omitted, document destinations keep the default companion (id_int) unless
// legacy primary_key_int overrides the name.
func ParsePrimaryKeyConfig(raw json.RawMessage) (PrimaryKeyConfig, error) {
	out := PrimaryKeyConfig{
		Destination: "id",
		Int:         DefaultSortableIDField,
	}
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return PrimaryKeyConfig{}, fmt.Errorf("parse sync job config: %w", err)
	}

	legacyInt := ""
	if v, ok := top["primary_key_int"]; ok {
		if err := json.Unmarshal(v, &legacyInt); err != nil {
			return PrimaryKeyConfig{}, fmt.Errorf("parse primary_key_int: %w", err)
		}
		legacyInt = strings.TrimSpace(legacyInt)
	}

	pkRaw, ok := top["primary_key"]
	if !ok || len(pkRaw) == 0 || string(pkRaw) == "null" {
		if legacyInt != "" {
			out.Int = legacyInt
		}
		return out, nil
	}

	out.Configured = true
	out.Int = "" // opt-in when primary_key is set
	if legacyInt != "" {
		out.Int = legacyInt
	}

	pkRaw = bytes.TrimSpace(pkRaw)
	switch {
	case len(pkRaw) > 0 && pkRaw[0] == '"':
		var s string
		if err := json.Unmarshal(pkRaw, &s); err != nil {
			return PrimaryKeyConfig{}, fmt.Errorf("parse primary_key: %w", err)
		}
		s = strings.TrimSpace(s)
		if s != "" {
			out.Source = []string{s}
		}
	case len(pkRaw) > 0 && pkRaw[0] == '[':
		var arr []string
		if err := json.Unmarshal(pkRaw, &arr); err != nil {
			return PrimaryKeyConfig{}, fmt.Errorf("parse primary_key: %w", err)
		}
		out.Source = trimNonEmpty(arr)
	case len(pkRaw) > 0 && pkRaw[0] == '{':
		var obj struct {
			Source      json.RawMessage `json:"source"`
			Destination string          `json:"destination"`
			Int         *string         `json:"int"`
		}
		if err := json.Unmarshal(pkRaw, &obj); err != nil {
			return PrimaryKeyConfig{}, fmt.Errorf("parse primary_key: %w", err)
		}
		src, err := parseSourceList(obj.Source)
		if err != nil {
			return PrimaryKeyConfig{}, err
		}
		out.Source = src
		if d := strings.TrimSpace(obj.Destination); d != "" {
			out.Destination = d
		}
		if obj.Int != nil {
			out.Int = strings.TrimSpace(*obj.Int)
		}
	default:
		return PrimaryKeyConfig{}, fmt.Errorf("parse primary_key: expected string, array, or object")
	}

	if out.Destination == "" {
		out.Destination = "id"
	}
	return out, nil
}

func parseSourceList(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse primary_key.source: %w", err)
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("parse primary_key.source: %w", err)
	}
	return trimNonEmpty(arr), nil
}

func trimNonEmpty(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ApplyPrimaryKeyOverride marks Source columns as primary keys on a copy of schema.
// No-op when cfg is not configured or Source is empty.
func ApplyPrimaryKeyOverride(schema *TableSchema, cfg PrimaryKeyConfig) (*TableSchema, error) {
	if !cfg.Configured || len(cfg.Source) == 0 || schema == nil {
		return schema, nil
	}
	want := map[string]struct{}{}
	for _, name := range cfg.Source {
		want[name] = struct{}{}
	}
	out := &TableSchema{Columns: make([]ColumnSchema, len(schema.Columns))}
	found := 0
	for i, col := range schema.Columns {
		mapped := col
		_, ok := want[col.Name]
		mapped.PrimaryKey = ok
		if ok {
			found++
		}
		out.Columns[i] = mapped
	}
	if found != len(want) {
		missing := make([]string, 0)
		have := map[string]struct{}{}
		for _, col := range out.Columns {
			if col.PrimaryKey {
				have[col.Name] = struct{}{}
			}
		}
		for _, name := range cfg.Source {
			if _, ok := have[name]; !ok {
				missing = append(missing, name)
			}
		}
		return nil, fmt.Errorf("primary_key source %v not found in source schema", missing)
	}
	return out, nil
}

// ApplyDestinationPrimaryKey sets Destination as the sole PK column on the destination schema.
// Adds the destination column when missing (string type — used for joined composite keys).
func ApplyDestinationPrimaryKey(schema *TableSchema, cfg PrimaryKeyConfig) *TableSchema {
	if !cfg.Configured || schema == nil {
		return schema
	}
	dest := strings.TrimSpace(cfg.Destination)
	if dest == "" {
		dest = "id"
	}
	out := &TableSchema{Columns: make([]ColumnSchema, 0, len(schema.Columns)+1)}
	found := false
	for _, col := range schema.Columns {
		mapped := col
		mapped.PrimaryKey = col.Name == dest
		if mapped.PrimaryKey {
			found = true
		}
		out.Columns = append(out.Columns, mapped)
	}
	if !found {
		out.Columns = append([]ColumnSchema{{
			Name:       dest,
			Type:       FieldTypeString,
			PrimaryKey: true,
		}}, out.Columns...)
	}
	return out
}
