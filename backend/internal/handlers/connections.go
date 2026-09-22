package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/services/connection"
)

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
