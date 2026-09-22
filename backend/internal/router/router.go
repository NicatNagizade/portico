package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/portico/backend/internal/handlers"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/portico/backend/docs"
)

func New(h *handlers.Handlers) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	// Direct listeners only; do not honor X-Forwarded-* from arbitrary clients.
	_ = r.SetTrustedProxies(nil)

	r.GET("/health", h.Health)

	r.GET("/connections", h.ListConnections)
	r.POST("/connections", h.CreateConnection)
	r.POST("/connections/check", h.CheckConnection)
	r.GET("/connections/:id", h.GetConnection)
	r.PUT("/connections/:id", h.UpdateConnection)
	r.DELETE("/connections/:id", h.DeleteConnection)
	r.GET("/connections/:id/tables", h.ListConnectionTables)
	r.GET("/connections/:id/columns", h.ListConnectionColumns)

	r.GET("/sync-jobs", h.ListSyncJobs)
	r.POST("/sync-jobs", h.CreateSyncJob)
	r.GET("/sync-jobs/:id", h.GetSyncJob)
	r.PUT("/sync-jobs/:id", h.UpdateSyncJob)
	r.DELETE("/sync-jobs/:id", h.DeleteSyncJob)
	r.POST("/sync-jobs/:id/run", h.RunSyncJob)
	r.POST("/sync-jobs/:id/start", h.StartSyncJob)
	r.POST("/sync-jobs/:id/explore", h.ExploreSyncJob)
	r.POST("/sync-jobs/:id/explore/export", h.ExportSyncJobExplore)

	r.GET("/sync-logs", h.ListSyncLogs)
	r.GET("/sync-logs/:id", h.GetSyncLog)
	r.POST("/sync-logs/:id/stop", h.StopSyncLog)

	r.GET("/api/documentation", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/api/documentation/index.html")
	})
	r.GET("/api/documentation/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return r
}
