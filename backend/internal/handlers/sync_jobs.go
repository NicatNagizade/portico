package handlers

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/portico/backend/internal/pagination"
	"github.com/portico/backend/internal/services/sync"
	"github.com/portico/backend/internal/services/syncjob"
)

// ListSyncJobs godoc
// @Summary List sync jobs
// @Description Returns a paginated list of sync jobs. Default page size is 20; maximum is 100.
// @Tags sync-jobs
// @Produce json
// @Param page query int false "Page number (1-based)" default(1) minimum(1)
// @Param page_size query int false "Items per page (max 100)" default(20) minimum(1) maximum(100)
// @Success 200 {object} SyncJobListResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs [get]
func (h *Handlers) ListSyncJobs(c *gin.Context) {
	p := pagination.Parse(c)
	items, total, err := h.SyncJobs.List(p.Page, p.PageSize)
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, pagination.NewPage(items, total, p))
}

// CreateSyncJob godoc
// @Summary Create sync job
// @Tags sync-jobs
// @Accept json
// @Produce json
// @Param body body syncjob.CreateInput true "Sync job"
// @Success 201 {object} models.SyncJob
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs [post]
func (h *Handlers) CreateSyncJob(c *gin.Context) {
	var in syncjob.CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	item, err := h.SyncJobs.Create(in)
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusCreated, item)
}

// GetSyncJob godoc
// @Summary Get sync job
// @Tags sync-jobs
// @Produce json
// @Param id path int true "Sync job ID"
// @Success 200 {object} models.SyncJob
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs/{id} [get]
func (h *Handlers) GetSyncJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.SyncJobs.Get(id)
	if err != nil {
		writeErr(c, err, syncjob.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, item)
}

// UpdateSyncJob godoc
// @Summary Update sync job
// @Tags sync-jobs
// @Accept json
// @Produce json
// @Param id path int true "Sync job ID"
// @Param body body syncjob.UpdateInput true "Sync job"
// @Success 200 {object} models.SyncJob
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs/{id} [put]
func (h *Handlers) UpdateSyncJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var in syncjob.UpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	item, err := h.SyncJobs.Update(id, in)
	if err != nil {
		writeErr(c, err, syncjob.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, item)
}

// DeleteSyncJob godoc
// @Summary Delete sync job
// @Tags sync-jobs
// @Param id path int true "Sync job ID"
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs/{id} [delete]
func (h *Handlers) DeleteSyncJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.SyncJobs.Delete(id); err != nil {
		writeErr(c, err, syncjob.ErrNotFound)
		return
	}
	c.Status(http.StatusNoContent)
}

// RunSyncJob godoc
// @Summary Run a sync job
// @Description Runs the sync synchronously and returns the finished sync log.
// @Tags sync-jobs
// @Produce json
// @Param id path int true "Sync job ID"
// @Success 200 {object} models.SyncLog
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs/{id}/run [post]
func (h *Handlers) RunSyncJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.SyncJobs.Get(id); err != nil {
		writeErr(c, err, syncjob.ErrNotFound)
		return
	}
	logEntry, err := h.Sync.Run(c.Request.Context(), id)
	if logEntry == nil {
		writeErr(c, err, nil)
		return
	}
	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError
	}
	c.JSON(status, logEntry)
}

// StartSyncJob godoc
// @Summary Start a sync job in the background
// @Description Creates a sync log with status running and returns it immediately. Poll GET /sync-logs/{id} for progress (rows_synced / rows_total) and final status.
// @Tags sync-jobs
// @Produce json
// @Param id path int true "Sync job ID"
// @Success 202 {object} models.SyncLog
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs/{id}/start [post]
func (h *Handlers) StartSyncJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.SyncJobs.Get(id); err != nil {
		writeErr(c, err, syncjob.ErrNotFound)
		return
	}
	logEntry, err := h.Sync.Start(id)
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusAccepted, logEntry)
}

type ExploreRequest struct {
	Side     string              `json:"side" binding:"required" example:"source"`
	Page     int                 `json:"page" example:"1"`
	PageSize int                 `json:"page_size" example:"50"`
	SortBy   string              `json:"sort_by" example:"id"`
	SortDir  string              `json:"sort_dir" example:"asc"`
	Filters  []sync.FilterInput  `json:"filters"`
}

type ExploreExportRequest struct {
	Side    string             `json:"side" binding:"required" example:"source"`
	Filters []sync.FilterInput `json:"filters"`
}

// ExploreSyncJob godoc
// @Summary Explore sync job data
// @Description Returns a paginated preview of source (rules/fields/relations applied) or destination documents for a sync job. Optional filters are AND'd (with job rules on source).
// @Tags sync-jobs
// @Accept json
// @Produce json
// @Param id path int true "Sync job ID"
// @Param body body ExploreRequest true "Explore options"
// @Success 200 {object} sync.PreviewResult
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs/{id}/explore [post]
func (h *Handlers) ExploreSyncJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req ExploreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid json body"})
		return
	}
	result, err := h.Sync.Preview(c.Request.Context(), id, req.Side, req.Page, req.PageSize, req.SortBy, req.SortDir, req.Filters)
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ExportSyncJobExplore godoc
// @Summary Export sync job explore data as CSV
// @Description Downloads matching source or destination rows as CSV. Optional filters match the explore preview.
// @Tags sync-jobs
// @Accept json
// @Produce text/csv
// @Param id path int true "Sync job ID"
// @Param body body ExploreExportRequest true "Export options"
// @Success 200 {string} string "CSV file"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs/{id}/explore/export [post]
func (h *Handlers) ExportSyncJobExplore(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req ExploreExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid json body"})
		return
	}
	var buf bytes.Buffer
	filename, err := h.Sync.ExportCSV(c.Request.Context(), id, req.Side, &buf, req.Filters)
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}
