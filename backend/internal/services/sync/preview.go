package sync

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/portico/backend/internal/connectors"
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
	ErrDestinationReadUnsupported = errors.New("destination does not support reading documents")
)

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

func (o *Orchestrator) loadJob(jobID uint) (*models.SyncJob, error) {
	var job models.SyncJob
	err := o.db.
		Preload("SourceConnection").
		Preload("DestinationConnection").
		Preload("Relations").
		Preload("Relations.Fields").
		Preload("Fields", "sync_job_relation_id IS NULL").
		Preload("Rules").
		First(&job, jobID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load sync job: %w", err)
	}
	return &job, nil
}

// Preview returns a paginated, job-shaped view of source or destination data.
func (o *Orchestrator) Preview(ctx context.Context, jobID uint, side string, page, pageSize int, sortBy, sortDir string) (*PreviewResult, error) {
	side = strings.TrimSpace(strings.ToLower(side))
	if side != SideSource && side != SideDestination {
		return nil, ErrInvalidSide
	}
	page, pageSize = normalizeExplorePaging(page, pageSize)
	sortBy, sortDir, order, err := normalizeExploreSort(sortBy, sortDir)
	if err != nil {
		return nil, err
	}

	job, err := o.loadJob(jobID)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	var rows []map[string]any
	var columns []string
	var total int64

	switch side {
	case SideSource:
		rows, columns, total, err = o.previewSource(ctx, job, pageSize, offset, order)
	case SideDestination:
		rows, columns, total, err = o.previewDestination(ctx, job, pageSize, offset, order)
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

func (o *Orchestrator) previewSource(ctx context.Context, job *models.SyncJob, limit, offset int, order *connectors.Order) ([]map[string]any, []string, int64, error) {
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

	sourceOrder, err := resolveSourceOrder(schema, job.Fields, order)
	if err != nil {
		return nil, nil, 0, err
	}

	filters := ActiveFilters(job.Rules)
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
	connectors.EnsureID(docs, schema, int64(offset))
	ApplyFields(docs, job.Fields)
	return docs, exploreColumns(schema, job, docs), total, nil
}

func (o *Orchestrator) previewDestination(ctx context.Context, job *models.SyncJob, limit, offset int, order *connectors.Order) ([]map[string]any, []string, int64, error) {
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

	rows, total, err := reader.Query(ctx, job.DestinationTable, limit, offset, order)
	if err != nil {
		return nil, nil, 0, err
	}
	return rows, exploreColumns(nil, job, rows), total, nil
}

// ExportCSV writes matching explore rows as CSV, reading in chunks so large tables work.
func (o *Orchestrator) ExportCSV(ctx context.Context, jobID uint, side string, w io.Writer) (filename string, err error) {
	side = strings.TrimSpace(strings.ToLower(side))
	if side != SideSource && side != SideDestination {
		return "", ErrInvalidSide
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
			rows, cols, _, err = o.previewSource(ctx, job, ExploreExportChunkSize, offset, nil)
		case SideDestination:
			rows, cols, _, err = o.previewDestination(ctx, job, ExploreExportChunkSize, offset, nil)
		}
		if err != nil {
			return "", err
		}
		if offset == 0 {
			headers = cols
			if headers == nil {
				headers = []string{}
			}
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
		out := SchemaWithFields(SchemaWithRelations(schema, job.Relations), job.Fields)
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
	for _, row := range rows {
		for k := range row {
			add(k)
		}
	}
	return cols
}

// resolveSourceOrder maps an explore sort field (destination/display name) to a
// source SQL column. Relation / object fields are rejected.
func resolveSourceOrder(schema *connectors.TableSchema, fields []models.SyncJobField, order *connectors.Order) (*connectors.Order, error) {
	if order == nil || strings.TrimSpace(order.Column) == "" {
		return nil, nil
	}
	want := strings.TrimSpace(order.Column)

	sourceName := ""
	for _, f := range fields {
		if !f.IsActive() || f.SourceName == "" {
			continue
		}
		dest := destinationFieldName(f)
		if dest == want || f.SourceName == want {
			sourceName = f.SourceName
			break
		}
	}
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
