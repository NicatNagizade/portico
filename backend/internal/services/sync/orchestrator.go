package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type Orchestrator struct {
	db       *gorm.DB
	registry *connectors.Registry
}

func NewOrchestrator(db *gorm.DB, registry *connectors.Registry) *Orchestrator {
	return &Orchestrator{db: db, registry: registry}
}

func (o *Orchestrator) Run(ctx context.Context, jobID uint) (*models.SyncLog, error) {
	started := time.Now()
	logEntry := models.SyncLog{
		SyncJobID: jobID,
		Status:    models.SyncLogStatusRunning,
		StartedAt: started,
		Message:   "sync started",
	}
	if err := o.db.Create(&logEntry).Error; err != nil {
		return nil, err
	}

	rowsTotal, rowsSynced, runErr := o.execute(ctx, jobID, logEntry.ID, started)
	finished := time.Now()
	duration := finished.Sub(started).Milliseconds()
	logEntry.FinishedAt = &finished
	logEntry.DurationMs = &duration
	logEntry.RowsTotal = &rowsTotal
	logEntry.RowsSynced = &rowsSynced

	if runErr != nil {
		logEntry.Status = models.SyncLogStatusFailed
		logEntry.Message = runErr.Error()
	} else {
		logEntry.Status = models.SyncLogStatusSuccess
		logEntry.Message = fmt.Sprintf("synced %d of %d rows", rowsSynced, rowsTotal)
	}

	if err := o.db.Save(&logEntry).Error; err != nil {
		return &logEntry, err
	}
	return &logEntry, runErr
}

func (o *Orchestrator) execute(ctx context.Context, jobID, logID uint, started time.Time) (rowsTotal, rowsSynced int64, err error) {
	var job models.SyncJob
	if err := o.db.
		Preload("SourceConnection").
		Preload("DestinationConnection").
		Preload("Relations").
		Preload("Relations.Fields").
		Preload("Fields", "sync_job_relation_id IS NULL").
		First(&job, jobID).Error; err != nil {
		return 0, 0, fmt.Errorf("load sync job: %w", err)
	}
	if job.SourceConnection == nil || job.DestinationConnection == nil {
		return 0, 0, fmt.Errorf("source or destination connection missing")
	}

	src, err := o.registry.NewSource(job.SourceConnection)
	if err != nil {
		return 0, 0, err
	}
	dst, err := o.registry.NewDestination(job.DestinationConnection)
	if err != nil {
		return 0, 0, err
	}

	if err := src.Open(ctx); err != nil {
		return 0, 0, fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	if err := dst.Open(ctx); err != nil {
		return 0, 0, fmt.Errorf("open destination: %w", err)
	}
	defer dst.Close()

	schema, err := src.Schema(ctx, job.SourceTable)
	if err != nil {
		return 0, 0, fmt.Errorf("introspect schema: %w", err)
	}
	outSchema := SchemaWithFields(SchemaWithRelations(schema, job.Relations), job.Fields)
	if err := dst.Prepare(ctx, job.DestinationTable, outSchema, json.RawMessage(job.Config)); err != nil {
		return 0, 0, fmt.Errorf("prepare destination: %w", err)
	}

	sourceTotal, err := src.Count(ctx, job.SourceTable)
	if err != nil {
		return 0, 0, fmt.Errorf("count source rows: %w", err)
	}
	if err := o.recordRowsTotal(logID, sourceTotal); err != nil {
		return 0, 0, fmt.Errorf("update rows_total: %w", err)
	}

	chunkSize := job.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 500
	}
	parallel := job.Workers
	if parallel <= 0 {
		parallel = 1
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(parallel)

	var syncedRows atomic.Int64
	var rowIndex atomic.Int64

	err = src.ReadChunks(gctx, job.SourceTable, chunkSize, func(docs []map[string]any) error {
		if err := gctx.Err(); err != nil {
			return err
		}
		batch := make([]map[string]any, len(docs))
		for i, doc := range docs {
			copied := make(map[string]any, len(doc))
			for k, v := range doc {
				copied[k] = v
			}
			batch[i] = copied
		}
		if err := enrichDocs(gctx, src, &job, schema, batch); err != nil {
			return err
		}
		start := rowIndex.Add(int64(len(batch))) - int64(len(batch))

		g.Go(func() error {
			connectors.EnsureID(batch, schema, start)
			ApplyFields(batch, job.Fields)
			if err := dst.WriteBatch(gctx, job.DestinationTable, batch); err != nil {
				return err
			}
			n := int64(len(batch))
			syncedRows.Add(n)
			if err := o.recordChunkSynced(logID, n, started); err != nil {
				return fmt.Errorf("update rows_synced: %w", err)
			}
			return nil
		})
		return nil
	})
	waitErr := g.Wait()
	rowsTotal = sourceTotal
	rowsSynced = syncedRows.Load()
	if waitErr != nil {
		return rowsTotal, rowsSynced, waitErr
	}
	if err != nil {
		return rowsTotal, rowsSynced, err
	}
	return rowsTotal, rowsSynced, nil
}

func (o *Orchestrator) recordRowsTotal(logID uint, n int64) error {
	return o.db.Model(&models.SyncLog{}).
		Where("id = ?", logID).
		UpdateColumn("rows_total", n).
		Error
}

func (o *Orchestrator) recordChunkSynced(logID uint, n int64, started time.Time) error {
	duration := time.Since(started).Milliseconds()
	return o.db.Model(&models.SyncLog{}).
		Where("id = ?", logID).
		Updates(map[string]any{
			"rows_synced": gorm.Expr("COALESCE(rows_synced, 0) + ?", n),
			"duration_ms": duration,
		}).
		Error
}
