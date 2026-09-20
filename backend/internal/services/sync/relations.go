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

// AssembleHasMany groups child rows by foreign key column.
func AssembleHasMany(childRows []map[string]any, foreignKey string) map[string][]map[string]any {
	out := make(map[string][]map[string]any)
	for _, row := range childRows {
		parentVal, ok := row[foreignKey]
		if !ok || parentVal == nil {
			continue
		}
		copied := make(map[string]any, len(row))
		for k, v := range row {
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
	parentTable string,
	parentSchema *connectors.TableSchema,
	docs []map[string]any,
	rel models.SyncJobRelation,
) error {
	rel = ResolveRelationKeys(parentTable, rel)
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

func enrichHasMany(
	ctx context.Context,
	src connectors.SourceReader,
	parentTable string,
	parentSchema *connectors.TableSchema,
	docs []map[string]any,
	rel models.SyncJobRelation,
) error {
	rel = ResolveRelationKeys(parentTable, rel)

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

	childRows, err := src.QueryRows(ctx, rel.Table, nil, rel.ForeignKey, ids)
	if err != nil {
		return fmt.Errorf("relation %q child query: %w", rel.Name, err)
	}

	grouped := AssembleHasMany(childRows, rel.ForeignKey)
	for _, doc := range docs {
		pid := fmt.Sprint(doc[parentPK])
		if rows, ok := grouped[pid]; ok {
			doc[rel.Name] = rows
		}
	}
	return nil
}

// orderRelationsByParent returns active relations in dependency order (parents before children).
func orderRelationsByParent(relations []models.SyncJobRelation) ([]models.SyncJobRelation, error) {
	active := make([]models.SyncJobRelation, 0, len(relations))
	byName := make(map[string]models.SyncJobRelation, len(relations))
	for _, rel := range relations {
		if !rel.IsActive() {
			continue
		}
		active = append(active, rel)
		byName[rel.Name] = rel
	}
	if len(active) == 0 {
		return nil, nil
	}

	depthOf := map[string]int{}
	var depth func(name string, stack map[string]struct{}) (int, error)
	depth = func(name string, stack map[string]struct{}) (int, error) {
		if d, ok := depthOf[name]; ok {
			return d, nil
		}
		if _, loop := stack[name]; loop {
			return 0, fmt.Errorf("cyclic parent_relation involving %q", name)
		}
		rel, ok := byName[name]
		if !ok {
			return 0, fmt.Errorf("parent_relation %q not found", name)
		}
		if rel.ParentRelation == "" {
			depthOf[name] = 0
			return 0, nil
		}
		stack[name] = struct{}{}
		parentDepth, err := depth(rel.ParentRelation, stack)
		delete(stack, name)
		if err != nil {
			return 0, err
		}
		d := parentDepth + 1
		depthOf[name] = d
		return d, nil
	}

	type scored struct {
		rel   models.SyncJobRelation
		depth int
		idx   int
	}
	scoredRels := make([]scored, 0, len(active))
	for i, rel := range active {
		d, err := depth(rel.Name, map[string]struct{}{})
		if err != nil {
			return nil, err
		}
		scoredRels = append(scoredRels, scored{rel: rel, depth: d, idx: i})
	}
	// Stable sort by depth, then original index.
	for i := 0; i < len(scoredRels); i++ {
		for j := i + 1; j < len(scoredRels); j++ {
			if scoredRels[j].depth < scoredRels[i].depth ||
				(scoredRels[j].depth == scoredRels[i].depth && scoredRels[j].idx < scoredRels[i].idx) {
				scoredRels[i], scoredRels[j] = scoredRels[j], scoredRels[i]
			}
		}
	}
	out := make([]models.SyncJobRelation, len(scoredRels))
	for i, s := range scoredRels {
		out[i] = s.rel
	}
	return out, nil
}

// collectRelationDocs returns all nested maps under relationName across root docs.
func collectRelationDocs(rootDocs []map[string]any, relationName string) []map[string]any {
	var out []map[string]any
	var walk func(doc map[string]any)
	walk = func(doc map[string]any) {
		for key, val := range doc {
			arr, ok := val.([]map[string]any)
			if !ok {
				continue
			}
			if key == relationName {
				out = append(out, arr...)
			}
			for _, child := range arr {
				walk(child)
			}
		}
	}
	for _, doc := range rootDocs {
		walk(doc)
	}
	return out
}

func relationByName(relations []models.SyncJobRelation, name string) (models.SyncJobRelation, bool) {
	for _, rel := range relations {
		if rel.Name == name {
			return rel, true
		}
	}
	return models.SyncJobRelation{}, false
}

func parentTableForRelation(job *models.SyncJob, rel models.SyncJobRelation) (string, error) {
	if rel.ParentRelation == "" {
		return job.SourceTable, nil
	}
	parent, ok := relationByName(job.Relations, rel.ParentRelation)
	if !ok {
		return "", fmt.Errorf("parent_relation %q not found for relation %q", rel.ParentRelation, rel.Name)
	}
	return parent.Table, nil
}

func enrichDocs(
	ctx context.Context,
	src connectors.SourceReader,
	job *models.SyncJob,
	parentSchema *connectors.TableSchema,
	docs []map[string]any,
) error {
	ordered, err := orderRelationsByParent(job.Relations)
	if err != nil {
		return err
	}

	schemaCache := map[string]*connectors.TableSchema{
		job.SourceTable: parentSchema,
	}

	for _, rel := range ordered {
		parentTable, err := parentTableForRelation(job, rel)
		if err != nil {
			return err
		}

		var targets []map[string]any
		var schema *connectors.TableSchema
		if rel.ParentRelation == "" {
			targets = docs
			schema = parentSchema
		} else {
			targets = collectRelationDocs(docs, rel.ParentRelation)
			if cached, ok := schemaCache[parentTable]; ok {
				schema = cached
			} else {
				schema, err = src.Schema(ctx, parentTable)
				if err != nil {
					return fmt.Errorf("relation %q parent schema: %w", rel.Name, err)
				}
				schemaCache[parentTable] = schema
			}
		}

		switch rel.Type {
		case models.RelationTypeBelongsToMany:
			if err := enrichBelongsToMany(ctx, src, parentTable, schema, targets, rel); err != nil {
				return err
			}
		case models.RelationTypeHasMany:
			if err := enrichHasMany(ctx, src, parentTable, schema, targets, rel); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported relation type %q", rel.Type)
		}
	}
	return nil
}

// SchemaWithRelations appends active root-level relation columns onto a base table schema.
// Nested relations (parent_relation set) are omitted; Typesense indexes them via enable_nested_fields.
func SchemaWithRelations(base *connectors.TableSchema, relations []models.SyncJobRelation) *connectors.TableSchema {
	out := &connectors.TableSchema{
		Columns: make([]connectors.ColumnSchema, len(base.Columns), len(base.Columns)+len(relations)),
	}
	copy(out.Columns, base.Columns)
	for _, rel := range relations {
		if !rel.IsActive() {
			continue
		}
		if rel.ParentRelation != "" {
			continue
		}
		out.Columns = append(out.Columns, connectors.ColumnSchema{
			Name: rel.Name,
			Type: connectors.FieldTypeObjectArray,
		})
	}
	return out
}
