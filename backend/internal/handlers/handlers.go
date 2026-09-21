package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
	"github.com/portico/backend/internal/pagination"
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
	Registry    *connectors.Registry
}

func New(
	connections *connection.Service,
	syncJobs *syncjob.Service,
	syncLogs *synclog.Service,
	orch *sync.Orchestrator,
	registry *connectors.Registry,
) *Handlers {
	return &Handlers{
		Connections: connections,
		SyncJobs:    syncJobs,
		SyncLogs:    syncLogs,
		Sync:        orch,
		Registry:    registry,
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// SyncJobListResponse is a paginated list of sync jobs.
type SyncJobListResponse struct {
	Items      []models.SyncJob `json:"items"`
	Page       int              `json:"page" example:"1"`
	PageSize   int              `json:"page_size" example:"20"`
	Total      int64            `json:"total" example:"42"`
	TotalPages int              `json:"total_pages" example:"3"`
}

// SyncLogListResponse is a paginated list of sync logs.
type SyncLogListResponse struct {
	Items      []models.SyncLog `json:"items"`
	Page       int              `json:"page" example:"1"`
	PageSize   int              `json:"page_size" example:"20"`
	Total      int64            `json:"total" example:"100"`
	TotalPages int              `json:"total_pages" example:"5"`
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

// CheckConnection godoc
// @Summary Check connection credentials
// @Description Opens the source or destination connector with the given config without saving. Pass id when editing so blank secrets keep the stored values.
// @Tags connections
// @Accept json
// @Produce json
// @Param body body connection.CheckInput true "Connection check"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /connections/check [post]
func (h *Handlers) CheckConnection(c *gin.Context) {
	var in connection.CheckInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	conn, err := h.Connections.PrepareCheck(in)
	if err != nil {
		writeErr(c, err, connection.ErrNotFound)
		return
	}
	if err := h.Registry.Check(c.Request.Context(), conn); err != nil {
		writeErr(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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

// ListConnectionTables godoc
// @Summary List tables for a source connection
// @Tags connections
// @Produce json
// @Param id path int true "Connection ID"
// @Success 200 {object} map[string][]string
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /connections/{id}/tables [get]
func (h *Handlers) ListConnectionTables(c *gin.Context) {
	src, ok := h.openSource(c)
	if !ok {
		return
	}
	defer src.Close()

	tables, err := src.ListTables(c.Request.Context())
	if err != nil {
		writeErr(c, err, nil)
		return
	}
	if tables == nil {
		tables = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"tables": tables})
}

// ColumnInfo is a source column name + mapped Portico field type.
type ColumnInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListConnectionColumns godoc
// @Summary List columns for a table on a source connection
// @Tags connections
// @Produce json
// @Param id path int true "Connection ID"
// @Param table query string true "Table name"
// @Success 200 {object} map[string][]ColumnInfo
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /connections/{id}/columns [get]
func (h *Handlers) ListConnectionColumns(c *gin.Context) {
	table := c.Query("table")
	if table == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "table is required"})
		return
	}

	src, ok := h.openSource(c)
	if !ok {
		return
	}
	defer src.Close()

	schema, err := src.Schema(c.Request.Context(), table)
	if err != nil {
		writeErr(c, err, nil)
		return
	}

	columns := make([]ColumnInfo, 0, len(schema.Columns))
	for _, col := range schema.Columns {
		columns = append(columns, ColumnInfo{
			Name: col.Name,
			Type: string(col.Type),
		})
	}
	c.JSON(http.StatusOK, gin.H{"columns": columns})
}

func (h *Handlers) openSource(c *gin.Context) (connectors.SourceReader, bool) {
	id, ok := pathID(c)
	if !ok {
		return nil, false
	}
	conn, err := h.Connections.Get(id)
	if err != nil {
		writeErr(c, err, connection.ErrNotFound)
		return nil, false
	}
	src, err := h.Registry.NewSource(conn)
	if err != nil {
		writeErr(c, err, nil)
		return nil, false
	}
	if err := src.Open(c.Request.Context()); err != nil {
		writeErr(c, err, nil)
		return nil, false
	}
	return src, true
}

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
