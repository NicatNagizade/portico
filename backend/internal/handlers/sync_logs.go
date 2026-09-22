package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/portico/backend/internal/pagination"
	"github.com/portico/backend/internal/services/sync"
	"github.com/portico/backend/internal/services/synclog"
)

// ListSyncLogs godoc
// @Summary List sync logs
// @Description Returns a paginated list of sync logs, newest first. Optionally filter by sync_job_id. Default page size is 20; maximum is 100. Each item includes the related sync job when available.
// @Tags sync-logs
// @Produce json
// @Param sync_job_id query int false "Filter by sync job ID"
// @Param page query int false "Page number (1-based)" default(1) minimum(1)
// @Param page_size query int false "Items per page (max 100)" default(20) minimum(1) maximum(100)
// @Success 200 {object} SyncLogListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-logs [get]
func (h *Handlers) ListSyncLogs(c *gin.Context) {
	var jobID *uint
	if raw := c.Query("sync_job_id"); raw != "" {
		id, err := parseID(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid sync_job_id"})
			return
		}
		jobID = &id
	}
	p := pagination.Parse(c)
	items, total, err := h.SyncLogs.List(jobID, p.Page, p.PageSize)
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, pagination.NewPage(items, total, p))
}

// GetSyncLog godoc
// @Summary Get sync log
// @Tags sync-logs
// @Produce json
// @Param id path int true "Sync log ID"
// @Success 200 {object} models.SyncLog
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-logs/{id} [get]
func (h *Handlers) GetSyncLog(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.SyncLogs.Get(id)
	if err != nil {
		writeErr(c, err, synclog.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, item)
}

// StopSyncLog godoc
// @Summary Stop a running sync log
// @Description Cancels a background sync and sets the log status to stopped.
// @Tags sync-logs
// @Produce json
// @Param id path int true "Sync log ID"
// @Success 200 {object} models.SyncLog
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-logs/{id}/stop [post]
func (h *Handlers) StopSyncLog(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.SyncLogs.Get(id); err != nil {
		writeErr(c, err, synclog.ErrNotFound)
		return
	}
	item, err := h.Sync.Stop(id)
	if err != nil {
		if errors.Is(err, sync.ErrNotRunning) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
			return
		}
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, item)
}
