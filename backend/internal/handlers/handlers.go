package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/portico/backend/internal/connectors"
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
	if errors.Is(err, syncjob.ErrInvalid) ||
		errors.Is(err, sync.ErrInvalidSide) ||
		errors.Is(err, sync.ErrInvalidSort) ||
		errors.Is(err, sync.ErrDestinationReadUnsupported) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if errors.Is(err, sync.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
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
