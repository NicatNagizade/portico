package sync

import (
	"context"
	"fmt"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
)

// Singularize strips a trailing "s" for simple English plural table names.
func Singularize(name string) string {
	if len(name) > 1 && strings.HasSuffix(name, "s") {
		return strings.TrimSuffix(name, "s")
	}
	return name
}

// ResolveRelationKeys fills empty foreign_key / related_key from table name conventions.
func ResolveRelationKeys(sourceTable string, rel models.SyncJobRelation) models.SyncJobRelation {
	if rel.ForeignKey == "" {
		rel.ForeignKey = Singularize(sourceTable) + "_id"
	}
	if rel.RelatedKey == "" {
		rel.RelatedKey = Singularize(rel.Table) + "_id"
	}
	return rel
}

func primaryKeyColumns(schema *connectors.TableSchema) []string {
	var pks []string
	for _, c := range schema.Columns {
		if c.PrimaryKey {
			pks = append(pks, c.Name)
		}
	}
	return pks
}

func primaryKeyColumn(schema *connectors.TableSchema) (string, error) {
	pks := primaryKeyColumns(schema)
	if len(pks) == 0 {
		return "", fmt.Errorf("table has no primary key")
	}
	if len(pks) > 1 {
		return "", fmt.Errorf("composite primary keys are not supported for related tables")
	}
	return pks[0], nil
}

func parentIDs(docs []map[string]any, pkCol string) []any {
	ids := make([]any, 0, len(docs))
	seen := make(map[string]struct{}, len(docs))
	for _, doc := range docs {
		v, ok := doc[pkCol]
		if !ok || v == nil {
			continue
		}
		key := fmt.Sprint(v)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, v)
	}
	return ids
}

// AssembleBelongsToMany builds parentID -> []relatedRow from separate pivot and related rows.
func AssembleBelongsToMany(
	pivotRows []map[string]any,
	relatedRows []map[string]any,
	foreignKey, relatedKey, relatedPK string,
) map[string][]map[string]any {
	relatedByID := make(map[string]map[string]any, len(relatedRows))
	for _, row := range relatedRows {
		if v, ok := row[relatedPK]; ok && v != nil {
			relatedByID[fmt.Sprint(v)] = row
		}
	}

	out := make(map[string][]map[string]any)
	for _, pivot := range pivotRows {
		parentVal, ok := pivot[foreignKey]
		if !ok || parentVal == nil {
			continue
		}
		relatedVal, ok := pivot[relatedKey]
		if !ok || relatedVal == nil {
			continue
		}
		related, ok := relatedByID[fmt.Sprint(relatedVal)]
		if !ok {
			continue
		}
		// Copy so callers can mutate safely.
		copied := make(map[string]any, len(related))
		for k, v := range related {
			copied[k] = v
		}
		parentID := fmt.Sprint(parentVal)
		out[parentID] = append(out[parentID], copied)
	}
	return out
}

func enrichBelongsToMany(
	ctx context.Context,
	src connectors.SourceReader,
	sourceTable string,
	parentSchema *connectors.TableSchema,
	docs []map[string]any,
	rel models.SyncJobRelation,
) error {
	rel = ResolveRelationKeys(sourceTable, rel)
	if rel.PivotTable == "" {
		return fmt.Errorf("relation %q: pivot_table is required", rel.Name)
	}

	parentPK, err := primaryKeyColumn(parentSchema)
	if err != nil {
		return fmt.Errorf("relation %q parent: %w", rel.Name, err)
	}
	ids := parentIDs(docs, parentPK)
	for _, doc := range docs {
		doc[rel.Name] = []map[string]any{}
	}
	if len(ids) == 0 {
		return nil
	}

	pivotRows, err := src.QueryRows(ctx, rel.PivotTable, []string{rel.ForeignKey, rel.RelatedKey}, rel.ForeignKey, ids)
	if err != nil {
		return fmt.Errorf("relation %q pivot query: %w", rel.Name, err)
	}

	relatedIDSet := make(map[string]any)
	for _, row := range pivotRows {
		if v, ok := row[rel.RelatedKey]; ok && v != nil {
			relatedIDSet[fmt.Sprint(v)] = v
		}
	}
	relatedIDs := make([]any, 0, len(relatedIDSet))
	for _, v := range relatedIDSet {
		relatedIDs = append(relatedIDs, v)
	}

	var relatedRows []map[string]any
	var relatedPK string
	if len(relatedIDs) > 0 {
		relatedSchema, err := src.Schema(ctx, rel.Table)
		if err != nil {
			return fmt.Errorf("relation %q related schema: %w", rel.Name, err)
		}
		relatedPK, err = primaryKeyColumn(relatedSchema)
		if err != nil {
			return fmt.Errorf("relation %q related: %w", rel.Name, err)
		}
		relatedRows, err = src.QueryRows(ctx, rel.Table, nil, relatedPK, relatedIDs)
		if err != nil {
			return fmt.Errorf("relation %q related query: %w", rel.Name, err)
		}
	}

	grouped := AssembleBelongsToMany(pivotRows, relatedRows, rel.ForeignKey, rel.RelatedKey, relatedPK)
	for _, doc := range docs {
		pid := fmt.Sprint(doc[parentPK])
		if rows, ok := grouped[pid]; ok {
			doc[rel.Name] = rows
		}
	}
	return nil
}

func enrichDocs(
	ctx context.Context,
	src connectors.SourceReader,
	job *models.SyncJob,
	parentSchema *connectors.TableSchema,
	docs []map[string]any,
) error {
	for _, rel := range job.Relations {
		if !rel.IsActive() {
			continue
		}
		switch rel.Type {
		case models.RelationTypeBelongsToMany:
			if err := enrichBelongsToMany(ctx, src, job.SourceTable, parentSchema, docs, rel); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported relation type %q", rel.Type)
		}
	}
	return nil
}

// SchemaWithRelations appends active relation columns onto a base table schema.
func SchemaWithRelations(base *connectors.TableSchema, relations []models.SyncJobRelation) *connectors.TableSchema {
	out := &connectors.TableSchema{
		Columns: make([]connectors.ColumnSchema, len(base.Columns), len(base.Columns)+len(relations)),
	}
	copy(out.Columns, base.Columns)
	for _, rel := range relations {
		if !rel.IsActive() {
			continue
		}
		out.Columns = append(out.Columns, connectors.ColumnSchema{
			Name: rel.Name,
			Type: connectors.FieldTypeObjectArray,
		})
	}
	return out
}
