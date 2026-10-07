package sync

import (
	"encoding/json"

	"github.com/portico/backend/internal/connectors"
)

// PrimaryKeyFromConfig parses optional primary_key settings from sync job config.
func PrimaryKeyFromConfig(raw json.RawMessage) (connectors.PrimaryKeyConfig, error) {
	return connectors.ParsePrimaryKeyConfig(raw)
}

// EffectiveSourceSchema applies optional primary_key source columns onto a source schema.
func EffectiveSourceSchema(schema *connectors.TableSchema, config json.RawMessage) (*connectors.TableSchema, error) {
	cfg, err := PrimaryKeyFromConfig(config)
	if err != nil {
		return nil, err
	}
	return connectors.ApplyPrimaryKeyOverride(schema, cfg)
}

// EffectiveDestinationSchema applies destination primary key naming after field/relation shaping.
func EffectiveDestinationSchema(schema *connectors.TableSchema, config json.RawMessage) (*connectors.TableSchema, error) {
	cfg, err := PrimaryKeyFromConfig(config)
	if err != nil {
		return nil, err
	}
	return connectors.ApplyDestinationPrimaryKey(schema, cfg), nil
}
