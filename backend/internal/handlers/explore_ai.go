package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/portico/backend/internal/llm"
	"github.com/portico/backend/internal/services/syncjob"
)

type ExploreSuggestRequest struct {
	Prompt       string   `json:"prompt" binding:"required" example:"active users named john; show id name email"`
	FilterFields []string `json:"filter_fields" example:"id,name,status,email"`
	Fields       []string `json:"fields" example:"id,name,email,status"`
}

// ExploreSuggestSyncJob godoc
// @Summary AI suggest explore filters and fields
// @Description Uses an OpenAI-compatible model to turn a natural-language request into explore filters and visible columns. Requires OPENAI_API_KEY.
// @Tags sync-jobs
// @Accept json
// @Produce json
// @Param id path int true "Sync job ID"
// @Param body body ExploreSuggestRequest true "Prompt and available field names"
// @Success 200 {object} llm.ExploreSuggestResult
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /sync-jobs/{id}/explore/suggest [post]
func (h *Handlers) ExploreSuggestSyncJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.SyncJobs.Get(id); err != nil {
		writeErr(c, err, syncjob.ErrNotFound)
		return
	}

	var req ExploreSuggestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid json body"})
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "prompt is required"})
		return
	}

	result, err := llm.SuggestExplore(c.Request.Context(), h.LLM, llm.ExploreSuggestInput{
		Prompt:       req.Prompt,
		FilterFields: req.FilterFields,
		Fields:       req.Fields,
	})
	if err != nil {
		if errors.Is(err, llm.ErrNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
			return
		}
		if strings.Contains(err.Error(), "prompt is required") || strings.Contains(err.Error(), "invalid ai json") {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
