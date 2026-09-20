package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Params struct {
	Page     int
	PageSize int
}

func Parse(c *gin.Context) Params {
	page := DefaultPage
	pageSize := DefaultPageSize

	if raw := c.Query("page"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			page = n
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return Params{Page: page, PageSize: pageSize}
}

func (p Params) Offset() int {
	return (p.Page - 1) * p.PageSize
}

func TotalPages(total int64, pageSize int) int {
	if pageSize <= 0 || total <= 0 {
		return 0
	}
	pages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		pages++
	}
	return pages
}

// Page is a JSON envelope for list endpoints.
type Page[T any] struct {
	Items      []T   `json:"items"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewPage[T any](items []T, total int64, p Params) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{
		Items:      items,
		Page:       p.Page,
		PageSize:   p.PageSize,
		Total:      total,
		TotalPages: TotalPages(total, p.PageSize),
	}
}
