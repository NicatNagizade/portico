package sync

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	"gorm.io/gorm"
)

const (
	SideSource      = "source"
	SideDestination = "destination"

	MaxExplorePageSize     = 100
	DefaultExplorePage     = 1
	DefaultExploreSize     = 50
	ExploreExportChunkSize = 250 // Typesense per_page max is 250; fine for SQL too
)

var (
	ErrNotFound                   = errors.New("sync job not found")
	ErrInvalidSide                = errors.New("side must be source or destination")
	ErrInvalidSort                = errors.New("invalid sort field or direction")
	ErrInvalidFilter              = errors.New("invalid explore filter")
	ErrDestinationReadUnsupported = errors.New("destination does not support reading documents")
)

// FilterInput is an explore-time predicate (AND'd with job rules on source).
type FilterInput struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// PreviewResult is a page of explore rows for a sync job.
type PreviewResult struct {
	Rows     []map[string]any `json:"rows"`
	Columns  []string         `json:"columns"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Side     string           `json:"side"`
	SortBy   string           `json:"sort_by,omitempty"`
	SortDir  string           `json:"sort_dir,omitempty"`
}

func normalizeExplorePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = DefaultExplorePage
	}
	if pageSize < 1 {
		pageSize = DefaultExploreSize
	}
	if pageSize > MaxExplorePageSize {
		pageSize = MaxExplorePageSize
	}
	return page, pageSize
}

func normalizeExploreSort(sortBy, sortDir string) (string, string, *connectors.Order, error) {
	sortBy = strings.TrimSpace(sortBy)
	sortDir = strings.TrimSpace(strings.ToLower(sortDir))
	if sortBy == "" {
		return "", "", nil, nil
	}
	if sortDir == "" {
		sortDir = "asc"
	}
	if sortDir != "asc" && sortDir != "desc" {
		return "", "", nil, ErrInvalidSort
	}
	return sortBy, sortDir, &connectors.Order{Column: sortBy, Desc: sortDir == "desc"}, nil
}

// primaryKeyName returns the first primary-key column, or "id" when unknown.
func primaryKeyName(schema *connectors.TableSchema) string {
	if schema != nil {
		if pks := primaryKeyColumns(schema); len(pks) > 0 && pks[0] != "" {
			return pks[0]
		}
	}
	return "id"
}

func (o *Orchestrator) loadJob(jobID uint) (*models.SyncJob, error) {
	var job models.SyncJob
	err := models.PreloadSyncJob(o.db).First(&job, jobID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load sync job: %w", err)
	}
	return &job, nil
}

// Preview returns a paginated, job-shaped view of source or destination data.
func (o *Orchestrator) Preview(ctx context.Context, jobID uint, side string, page, pageSize int, sortBy, sortDir string, exploreFilters []FilterInput) (*PreviewResult, error) {
	side = strings.TrimSpace(strings.ToLower(side))
	if side != SideSource && side != SideDestination {
		return nil, ErrInvalidSide
	}
	page, pageSize = normalizeExplorePaging(page, pageSize)
	sortBy, sortDir, order, err := normalizeExploreSort(sortBy, sortDir)
	if err != nil {
		return nil, err
	}
	extra, err := parseExploreFilters(exploreFilters)
	if err != nil {
		return nil, err
	}

	job, err := o.loadJob(jobID)
	if err != nil {
		return nil, err
	}

	// Default sort: source primary key, or document id on destination.
	if order == nil {
		pk := "id"
		if side == SideSource {
			pk = primaryKeyName(o.sourceSchema(ctx, job))
		}
		sortBy, sortDir, order, err = normalizeExploreSort(pk, "asc")
		if err != nil {
			return nil, err
		}
	}

	offset := (page - 1) * pageSize
	var rows []map[string]any
	var columns []string
	var total int64

	switch side {
	case SideSource:
		rows, columns, total, err = o.previewSource(ctx, job, pageSize, offset, order, extra)
	case SideDestination:
		rows, columns, total, err = o.previewDestination(ctx, job, pageSize, offset, order, extra)
	}
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	if columns == nil {
		columns = []string{}
	}
	return &PreviewResult{
		Rows:     rows,
		Columns:  columns,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Side:     side,
		SortBy:   sortBy,
		SortDir:  sortDir,
	}, nil
}

func (o *Orchestrator) previewSource(ctx context.Context, job *models.SyncJob, limit, offset int, order *connectors.Order, extra []connectors.Filter) ([]map[string]any, []string, int64, error) {
	if job.SourceConnection == nil {
		return nil, nil, 0, fmt.Errorf("source connection missing")
	}
	src, err := o.registry.NewSource(job.SourceConnection)
	if err != nil {
		return nil, nil, 0, err
	}
	if err := src.Open(ctx); err != nil {
		return nil, nil, 0, fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	schema, err := src.Schema(ctx, job.SourceTable)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("introspect schema: %w", err)
	}
	schema, err = EffectiveSourceSchema(schema, json.RawMessage(job.Config))
	if err != nil {
		return nil, nil, 0, err
	}

	displayOrder := order
	sourceOrder, err := resolveSourceOrder(schema, job.Fields, order)
	if err != nil {
		return nil, nil, 0, err
	}

	extra = resolveSourceFilters(job.Fields, extra)
	rootExtra, nestedExtra := splitNestedFilters(extra)
	filters := append(ActiveFilters(job.Rules), rootExtra...)

	// Nested paths (user.bio) map through sync-job relations to GORM IN-subqueries
	// (belongs_to → user_id IN (SELECT id FROM users WHERE bio = ?)).
	if len(nestedExtra) > 0 {
		if _, ok := src.(connectors.RelationFilterSupport); ok {
			pushed, remaining, err := pushNestedFilters(ctx, src, job, schema, nestedExtra)
			if err != nil {
				return nil, nil, 0, err
			}
			filters = append(filters, pushed...)
			if len(remaining) > 0 {
				return previewSourceNested(ctx, src, job, schema, filters, remaining, displayOrder, limit, offset)
			}
		} else {
			return previewSourceNested(ctx, src, job, schema, filters, nestedExtra, displayOrder, limit, offset)
		}
	}

	total, err := src.Count(ctx, job.SourceTable, filters)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("count source rows: %w", err)
	}

	docs, err := src.Query(ctx, job.SourceTable, nil, filters, limit, offset, sourceOrder)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("query source rows: %w", err)
	}
	if err := enrichDocs(ctx, src, job, schema, docs); err != nil {
		return nil, nil, 0, err
	}
	pk, err := PrimaryKeyFromConfig(json.RawMessage(job.Config))
	if err != nil {
		return nil, nil, 0, err
	}
	connectors.EnsureID(docs, schema, int64(offset), pk.Destination)
	ApplyFields(docs, job.Fields)
	return docs, exploreColumns(schema, job, docs), total, nil
}

// previewSourceNested loads root rows matching SQL filters, enriches relations,
// applies dotted-path filters in memory, then sorts and pages.
func previewSourceNested(
	ctx context.Context,
	src connectors.SourceReader,
	job *models.SyncJob,
	schema *connectors.TableSchema,
	rootFilters, nestedFilters []connectors.Filter,
	order *connectors.Order,
	limit, offset int,
) ([]map[string]any, []string, int64, error) {
	var matched []map[string]any
	err := src.ReadChunks(ctx, job.SourceTable, ExploreExportChunkSize, rootFilters, func(batch []map[string]any) error {
		docs := docutil.CloneRows(batch)
		if err := enrichDocs(ctx, src, job, schema, docs); err != nil {
			return err
		}
		pk, err := PrimaryKeyFromConfig(json.RawMessage(job.Config))
		if err != nil {
			return err
		}
		connectors.EnsureID(docs, schema, 0, pk.Destination)
		ApplyFields(docs, job.Fields)
		matched = append(matched, docutil.FilterRows(docs, nestedFilters)...)
		return nil
	})
	if err != nil {
		return nil, nil, 0, fmt.Errorf("query source rows: %w", err)
	}
	docutil.SortRows(matched, order)
	total := int64(len(matched))
	page := docutil.PageRows(matched, limit, offset)
	return page, exploreColumns(schema, job, page), total, nil
}

func splitNestedFilters(filters []connectors.Filter) (root, nested []connectors.Filter) {
	for _, f := range filters {
		if strings.Contains(f.Column, ".") {
			nested = append(nested, f)
		} else {
			root = append(root, f)
		}
	}
	return root, nested
}

func canPushNestedOperator(op string) bool {
	switch op {
	case models.RuleOperatorEq, models.RuleOperatorIn, models.RuleOperatorLike,
		models.RuleOperatorGt, models.RuleOperatorGte, models.RuleOperatorLt, models.RuleOperatorLte,
		models.RuleOperatorIsNotNull:
		return true
	default:
		return false
	}
}

// pushNestedFilters turns one-hop relation filters (user.bio) into root filters with
// RelationSubquery (GORM: root.fk IN (SELECT … FROM related WHERE …)).
func pushNestedFilters(
	ctx context.Context,
	src connectors.SourceReader,
	job *models.SyncJob,
	parentSchema *connectors.TableSchema,
	nested []connectors.Filter,
) (pushed, remaining []connectors.Filter, err error) {
	byName := map[string]models.SyncJobRelation{}
	for _, rel := range job.Relations {
		if !rel.IsActive() || rel.ParentID != nil || rel.Name == "" {
			continue
		}
		byName[rel.Name] = rel
	}

	for _, f := range nested {
		relName, rest, ok := strings.Cut(f.Column, ".")
		if !ok || rest == "" || strings.Contains(rest, ".") || !canPushNestedOperator(f.Operator) {
			remaining = append(remaining, f)
			continue
		}
		rel, ok := byName[relName]
		if !ok {
			remaining = append(remaining, f)
			continue
		}
		col := mapDisplayToSource(rel.Fields, rest)
		if col == "" {
			col = rest
		}
		relatedFilter := connectors.Filter{Column: col, Operator: f.Operator, Value: f.Value}
		rootFilter, ok, pushErr := relationSubqueryFilter(ctx, src, job, parentSchema, rel, relatedFilter)
		if pushErr != nil {
			return nil, nil, pushErr
		}
		if !ok {
			remaining = append(remaining, f)
			continue
		}
		pushed = append(pushed, *rootFilter)
	}
	return pushed, remaining, nil
}

func relationSubqueryFilter(
	ctx context.Context,
	src connectors.SourceReader,
	job *models.SyncJob,
	parentSchema *connectors.TableSchema,
	rel models.SyncJobRelation,
	relatedFilter connectors.Filter,
) (*connectors.Filter, bool, error) {
	rel = ResolveRelationKeys(job.SourceTable, rel)

	switch rel.Type {
	case models.RelationTypeBelongsTo:
		relatedSchema, err := src.Schema(ctx, rel.Table)
		if err != nil {
			return nil, false, fmt.Errorf("relation %q schema: %w", rel.Name, err)
		}
		ownerKey := rel.RelatedKey
		if ownerKey == "" {
			ownerKey, err = primaryKeyColumn(relatedSchema)
			if err != nil {
				return nil, false, fmt.Errorf("relation %q related: %w", rel.Name, err)
			}
		}
		return &connectors.Filter{
			Column: rel.ForeignKey,
			Rel: &connectors.RelationSubquery{
				Table:  rel.Table,
				Select: ownerKey,
				Where:  []connectors.Filter{relatedFilter},
			},
		}, true, nil

	case models.RelationTypeHasMany, models.RelationTypeHasOne:
		localKey := rel.RelatedKey
		if localKey == "" {
			pk, err := primaryKeyColumn(parentSchema)
			if err != nil {
				return nil, false, fmt.Errorf("relation %q parent: %w", rel.Name, err)
			}
			localKey = pk
		}
		return &connectors.Filter{
			Column: localKey,
			Rel: &connectors.RelationSubquery{
				Table:  rel.Table,
				Select: rel.ForeignKey,
				Where:  []connectors.Filter{relatedFilter},
			},
		}, true, nil

	case models.RelationTypeBelongsToMany:
		pivotTable := relationPivotTable(rel)
		if pivotTable == "" {
			return nil, false, nil
		}
		relatedSchema, err := src.Schema(ctx, rel.Table)
		if err != nil {
			return nil, false, fmt.Errorf("relation %q schema: %w", rel.Name, err)
		}
		relatedPK, err := primaryKeyColumn(relatedSchema)
		if err != nil {
			return nil, false, fmt.Errorf("relation %q related: %w", rel.Name, err)
		}
		parentPK, err := primaryKeyColumn(parentSchema)
		if err != nil {
			return nil, false, fmt.Errorf("relation %q parent: %w", rel.Name, err)
		}
		return &connectors.Filter{
			Column: parentPK,
			Rel: &connectors.RelationSubquery{
				Table:  pivotTable,
				Select: rel.ForeignKey,
				Where: []connectors.Filter{{
					Column: rel.RelatedKey,
					Rel: &connectors.RelationSubquery{
						Table:  rel.Table,
						Select: relatedPK,
						Where:  []connectors.Filter{relatedFilter},
					},
				}},
			},
		}, true, nil

	default:
		return nil, false, nil
	}
}

func (o *Orchestrator) previewDestination(ctx context.Context, job *models.SyncJob, limit, offset int, order *connectors.Order, filters []connectors.Filter) ([]map[string]any, []string, int64, error) {
	if job.DestinationConnection == nil {
		return nil, nil, 0, fmt.Errorf("destination connection missing")
	}
	dst, err := o.registry.NewDestination(job.DestinationConnection)
	if err != nil {
		return nil, nil, 0, err
	}
	reader, ok := dst.(connectors.DestinationReader)
	if !ok {
		_ = dst.Close()
		return nil, nil, 0, ErrDestinationReadUnsupported
	}
	if err := reader.Open(ctx); err != nil {
		return nil, nil, 0, fmt.Errorf("open destination: %w", err)
	}
	defer reader.Close()

	if cfg, ok := dst.(interface{ ApplyJobConfig(json.RawMessage) error }); ok {
		if err := cfg.ApplyJobConfig(json.RawMessage(job.Config)); err != nil {
			return nil, nil, 0, err
		}
	}

	rows, total, err := reader.Query(ctx, job.DestinationTable, filters, limit, offset, order)
	if err != nil {
		return nil, nil, 0, err
	}
	// Use source schema so column order matches sync (not random map key order from Typesense docs).
	return rows, exploreColumns(o.sourceSchema(ctx, job), job, rows), total, nil
}

// sourceSchema introspects the job's source table. Best-effort — nil on failure.
func (o *Orchestrator) sourceSchema(ctx context.Context, job *models.SyncJob) *connectors.TableSchema {
	if job == nil || job.SourceConnection == nil {
		return nil
	}
	src, err := o.registry.NewSource(job.SourceConnection)
	if err != nil {
		return nil
	}
	if err := src.Open(ctx); err != nil {
		return nil
	}
	defer src.Close()
	schema, err := src.Schema(ctx, job.SourceTable)
	if err != nil {
		return nil
	}
	schema, err = EffectiveSourceSchema(schema, json.RawMessage(job.Config))
	if err != nil {
		return nil
	}
	return schema
}

// ExportCSV writes matching explore rows as CSV, reading in chunks so large tables work.
// fields nil = all columns; non-nil (including empty) = only those columns that exist, in natural order.
func (o *Orchestrator) ExportCSV(ctx context.Context, jobID uint, side string, w io.Writer, exploreFilters []FilterInput, fields []string) (filename string, err error) {
	side = strings.TrimSpace(strings.ToLower(side))
	if side != SideSource && side != SideDestination {
		return "", ErrInvalidSide
	}
	extra, err := parseExploreFilters(exploreFilters)
	if err != nil {
		return "", err
	}

	job, err := o.loadJob(jobID)
	if err != nil {
		return "", err
	}

	cw := csv.NewWriter(w)
	var headers []string
	for offset := 0; ; offset += ExploreExportChunkSize {
		var rows []map[string]any
		var cols []string
		switch side {
		case SideSource:
			rows, cols, _, err = o.previewSource(ctx, job, ExploreExportChunkSize, offset, nil, extra)
		case SideDestination:
			rows, cols, _, err = o.previewDestination(ctx, job, ExploreExportChunkSize, offset, nil, extra)
		}
		if err != nil {
			return "", err
		}
		if offset == 0 {
			headers = filterExportColumns(cols, fields)
			if err := cw.Write(headers); err != nil {
				return "", err
			}
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			record := make([]string, len(headers))
			for i, h := range headers {
				record[i] = csvCell(row[h])
			}
			if err := cw.Write(record); err != nil {
				return "", err
			}
		}
		if len(rows) < ExploreExportChunkSize {
			break
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return "", err
	}

	safeName := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, job.Name)
	if safeName == "" {
		safeName = fmt.Sprintf("sync-job-%d", job.ID)
	}
	return fmt.Sprintf("%s-%s.csv", safeName, side), nil
}

// filterExportColumns keeps natural column order. fields nil = all; otherwise only matching names.
func filterExportColumns(cols []string, fields []string) []string {
	if fields == nil {
		if cols == nil {
			return []string{}
		}
		return cols
	}
	want := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		want[f] = struct{}{}
	}
	out := make([]string, 0, len(want))
	for _, c := range cols {
		if _, ok := want[c]; ok {
			out = append(out, c)
		}
	}
	return out
}

// exploreColumns returns field names in schema/relation/field-mapping order,
// then any extra keys present on rows (e.g. EnsureID-added id).
func exploreColumns(schema *connectors.TableSchema, job *models.SyncJob, rows []map[string]any) []string {
	seen := map[string]struct{}{}
	var cols []string
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		cols = append(cols, name)
	}

	if schema != nil && job != nil {
		out := SchemaWithFields(SchemaWithRelations(schema, job.Relations, nil), job.Fields)
		if out != nil {
			for _, c := range out.Columns {
				add(c.Name)
			}
		}
	} else if job != nil {
		for _, f := range job.Fields {
			if f.IsActive() {
				add(destinationFieldName(f))
			}
		}
		for _, rel := range job.Relations {
			if rel.IsActive() && rel.ParentID == nil {
				add(rel.Name)
			}
		}
	}
	// Map key iteration order is random — collect leftovers then sort for stable UI columns.
	var extra []string
	for _, row := range rows {
		for k := range row {
			if k == "" {
				continue
			}
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	cols = append(cols, extra...)
	return cols
}

// resolveSourceOrder maps an explore sort field (destination/display name) to a
// source SQL column. Relation / object fields are rejected.
func resolveSourceOrder(schema *connectors.TableSchema, fields []models.SyncJobField, order *connectors.Order) (*connectors.Order, error) {
	if order == nil || strings.TrimSpace(order.Column) == "" {
		return nil, nil
	}
	want := strings.TrimSpace(order.Column)

	sourceName := mapDisplayToSource(fields, want)
	if sourceName == "" {
		sourceName = want
	}

	if schema != nil {
		for _, c := range schema.Columns {
			if c.Name != sourceName {
				continue
			}
			if c.Type == connectors.FieldTypeObject || c.Type == connectors.FieldTypeObjectArray {
				return nil, ErrInvalidSort
			}
			return &connectors.Order{Column: sourceName, Desc: order.Desc}, nil
		}
	}
	// Allow sorting by a known field override even if schema lookup missed it.
	for _, f := range fields {
		if f.IsActive() && f.SourceName == sourceName {
			return &connectors.Order{Column: sourceName, Desc: order.Desc}, nil
		}
	}
	return nil, ErrInvalidSort
}

func parseExploreFilters(in []FilterInput) ([]connectors.Filter, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]connectors.Filter, 0, len(in))
	for _, f := range in {
		field := strings.TrimSpace(f.Field)
		op := strings.TrimSpace(f.Operator)
		if field == "" {
			return nil, fmt.Errorf("%w: field is required", ErrInvalidFilter)
		}
		if !models.ValidRuleOperator(op) {
			return nil, fmt.Errorf("%w: unsupported operator %q", ErrInvalidFilter, op)
		}
		value := f.Value
		if models.RuleNeedsValue(op) {
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("%w: value is required for operator %q", ErrInvalidFilter, op)
			}
		} else {
			value = ""
		}
		out = append(out, connectors.Filter{Column: field, Operator: op, Value: value})
	}
	return out, nil
}

// resolveSourceFilters maps display/destination field names to source columns.
func resolveSourceFilters(fields []models.SyncJobField, filters []connectors.Filter) []connectors.Filter {
	if len(filters) == 0 {
		return nil
	}
	out := make([]connectors.Filter, len(filters))
	for i, f := range filters {
		col := mapDisplayToSource(fields, f.Column)
		if col == "" {
			col = f.Column
		}
		out[i] = connectors.Filter{Column: col, Operator: f.Operator, Value: f.Value}
	}
	return out
}

func mapDisplayToSource(fields []models.SyncJobField, want string) string {
	want = strings.TrimSpace(want)
	if want == "" {
		return ""
	}
	for _, f := range fields {
		if !f.IsActive() || f.SourceName == "" {
			continue
		}
		dest := destinationFieldName(f)
		if dest == want || f.SourceName == want {
			return f.SourceName
		}
	}
	return ""
}

func csvCell(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case map[string]any, []any, []map[string]any:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(b)
	default:
		return fmt.Sprint(v)
	}
}
