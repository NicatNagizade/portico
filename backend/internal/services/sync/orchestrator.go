package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdsync "sync"
	"sync/atomic"
	"time"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

var ErrNotRunning = errors.New("sync log is not running")

type Orchestrator struct {
	db       *gorm.DB
	registry *connectors.Registry
	mu       stdsync.Mutex
	cancels  map[uint]context.CancelFunc
}

func NewOrchestrator(db *gorm.DB, registry *connectors.Registry) *Orchestrator {
	return &Orchestrator{
		db:       db,
		registry: registry,
		cancels:  make(map[uint]context.CancelFunc),
	}
}

func (o *Orchestrator) registerCancel(logID uint, cancel context.CancelFunc) {
	o.mu.Lock()
	o.cancels[logID] = cancel
	o.mu.Unlock()
}

func (o *Orchestrator) unregisterCancel(logID uint) {
	o.mu.Lock()
	delete(o.cancels, logID)
	o.mu.Unlock()
}

func (o *Orchestrator) createRunningLog(jobID uint) (*models.SyncLog, time.Time, error) {
	started := time.Now()
	logEntry := models.SyncLog{
		SyncJobID: jobID,
		Status:    models.SyncLogStatusRunning,
		StartedAt: started,
		Message:   "sync started",
	}
	if err := o.db.Create(&logEntry).Error; err != nil {
		return nil, time.Time{}, err
	}
	return &logEntry, started, nil
}

func (o *Orchestrator) finalize(logID uint, started time.Time, rowsTotal, rowsSynced int64, runErr error) (*models.SyncLog, error) {
	var logEntry models.SyncLog
	if err := o.db.First(&logEntry, logID).Error; err != nil {
		return nil, err
	}
	finished := time.Now()
	duration := finished.Sub(started).Milliseconds()
	logEntry.DurationMs = &duration
	logEntry.RowsTotal = &rowsTotal
	logEntry.RowsSynced = &rowsSynced

	if logEntry.Status != models.SyncLogStatusRunning {
		// Stop() already finalized this log; keep its status/message and refresh counts.
		if logEntry.FinishedAt == nil {
			logEntry.FinishedAt = &finished
		}
		if err := o.db.Save(&logEntry).Error; err != nil {
			return &logEntry, err
		}
		return &logEntry, runErr
	}

	logEntry.FinishedAt = &finished
	switch {
	case errors.Is(runErr, context.Canceled):
		logEntry.Status = models.SyncLogStatusStopped
		logEntry.Message = "sync stopped"
	case runErr != nil:
		logEntry.Status = models.SyncLogStatusFailed
		logEntry.Message = runErr.Error()
	default:
		logEntry.Status = models.SyncLogStatusSuccess
		logEntry.Message = fmt.Sprintf("synced %d of %d rows", rowsSynced, rowsTotal)
	}
	if err := o.db.Save(&logEntry).Error; err != nil {
		return &logEntry, err
	}
	return &logEntry, runErr
}

// Start creates a running sync log and executes the sync in the background.
func (o *Orchestrator) Start(jobID uint) (*models.SyncLog, error) {
	logEntry, started, err := o.createRunningLog(jobID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	o.registerCancel(logEntry.ID, cancel)
	go func() {
		defer func() {
			o.unregisterCancel(logEntry.ID)
			cancel()
		}()
		rowsTotal, rowsSynced, runErr := o.execute(ctx, jobID, logEntry.ID, started)
		_, _ = o.finalize(logEntry.ID, started, rowsTotal, rowsSynced, runErr)
	}()
	return logEntry, nil
}

// Run creates a sync log and executes the sync synchronously until it finishes.
func (o *Orchestrator) Run(ctx context.Context, jobID uint) (*models.SyncLog, error) {
	logEntry, started, err := o.createRunningLog(jobID)
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	o.registerCancel(logEntry.ID, cancel)
	defer func() {
		o.unregisterCancel(logEntry.ID)
		cancel()
	}()
	rowsTotal, rowsSynced, runErr := o.execute(runCtx, jobID, logEntry.ID, started)
	return o.finalize(logEntry.ID, started, rowsTotal, rowsSynced, runErr)
}

// Stop cancels a running sync and marks the log as stopped.
func (o *Orchestrator) Stop(logID uint) (*models.SyncLog, error) {
	var logEntry models.SyncLog
	if err := o.db.First(&logEntry, logID).Error; err != nil {
		return nil, err
	}
	if logEntry.Status != models.SyncLogStatusRunning {
		return nil, ErrNotRunning
	}

	o.mu.Lock()
	cancel, ok := o.cancels[logID]
	o.mu.Unlock()
	if ok {
		cancel()
	}

	finished := time.Now()
	duration := finished.Sub(logEntry.StartedAt).Milliseconds()
	logEntry.Status = models.SyncLogStatusStopped
	logEntry.Message = "sync stopped"
	logEntry.FinishedAt = &finished
	logEntry.DurationMs = &duration
	if err := o.db.Save(&logEntry).Error; err != nil {
		return &logEntry, err
	}
	return &logEntry, nil
}

func (o *Orchestrator) execute(ctx context.Context, jobID, logID uint, started time.Time) (rowsTotal, rowsSynced int64, err error) {
	var job models.SyncJob
	if err := o.db.
		Preload("SourceConnection").
		Preload("DestinationConnection").
		Preload("Relations").
		Preload("Relations.Fields").
		Preload("Relations.Fields.Values").
		Preload("Fields", "sync_job_relation_id IS NULL").
		Preload("Fields.Values").
		Preload("Rules").
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

	filters := ActiveFilters(job.Rules)
	sourceTotal, err := src.Count(ctx, job.SourceTable, filters)
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

	err = src.ReadChunks(gctx, job.SourceTable, chunkSize, filters, func(docs []map[string]any) error {
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
