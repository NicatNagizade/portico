package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/portico/backend/internal/models"
	"github.com/portico/backend/internal/services/connection"
	"github.com/portico/backend/internal/services/sync"
	"github.com/portico/backend/internal/services/syncjob"
	"github.com/portico/backend/internal/services/synclog"
)

// Keep models imported for swag type resolution.
var _ = models.Connection{}

type Handlers struct {
	Connections *connection.Service
	SyncJobs    *syncjob.Service
	SyncLogs    *synclog.Service
	Sync        *sync.Orchestrator
}

func New(
	connections *connection.Service,
	syncJobs *syncjob.Service,
	syncLogs *synclog.Service,
	orch *sync.Orchestrator,
) *Handlers {
	return &Handlers{
		Connections: connections,
		SyncJobs:    syncJobs,
		SyncLogs:    syncLogs,
		Sync:        orch,
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func parseID(raw string) (uint, error) {
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}

func pathID(c *gin.Context) (uint, bool) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return 0, false
	}
	return id, true
}

func writeErr(c *gin.Context, err error, notFound error) {
	if notFound != nil && errors.Is(err, notFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}
	if errors.Is(err, syncjob.ErrInvalid) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
}

// Health godoc
// @Summary Health check
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListConnections godoc
// @Summary List connections
// @Tags connections
// @Produce json
// @Success 200 {array} models.Connection
// @Failure 500 {object} ErrorResponse
// @Router /connections [get]
func (h *Handlers) ListConnections(c *gin.Context) {
	items, err := h.Connections.List()
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, items)
}

// CreateConnection godoc
// @Summary Create connection
// @Tags connections
// @Accept json
// @Produce json
// @Param body body connection.CreateInput true "Connection"
// @Success 201 {object} models.Connection
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /connections [post]
func (h *Handlers) CreateConnection(c *gin.Context) {
	var in connection.CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	item, err := h.Connections.Create(in)
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusCreated, item)
}

// GetConnection godoc
// @Summary Get connection
// @Tags connections
// @Produce json
// @Param id path int true "Connection ID"
// @Success 200 {object} models.Connection
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /connections/{id} [get]
func (h *Handlers) GetConnection(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.Connections.Get(id)
	if err != nil {
		writeErr(c, err, connection.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, item)
}

// UpdateConnection godoc
// @Summary Update connection
// @Tags connections
// @Accept json
// @Produce json
// @Param id path int true "Connection ID"
// @Param body body connection.UpdateInput true "Connection"
// @Success 200 {object} models.Connection
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /connections/{id} [put]
func (h *Handlers) UpdateConnection(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var in connection.UpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	item, err := h.Connections.Update(id, in)
	if err != nil {
		writeErr(c, err, connection.ErrNotFound)
		return
	}
	c.JSON(http.StatusOK, item)
}

// DeleteConnection godoc
// @Summary Delete connection
// @Tags connections
// @Param id path int true "Connection ID"
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /connections/{id} [delete]
func (h *Handlers) DeleteConnection(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Connections.Delete(id); err != nil {
		writeErr(c, err, connection.ErrNotFound)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListSyncJobs godoc
// @Summary List sync jobs
// @Tags sync-jobs
// @Produce json
// @Success 200 {array} models.SyncJob
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs [get]
func (h *Handlers) ListSyncJobs(c *gin.Context) {
	items, err := h.SyncJobs.List()
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, items)
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

// ListSyncLogs godoc
// @Summary List sync logs
// @Tags sync-logs
// @Produce json
// @Param sync_job_id query int false "Filter by sync job ID"
// @Success 200 {array} models.SyncLog
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
	items, err := h.SyncLogs.List(jobID)
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, items)
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
