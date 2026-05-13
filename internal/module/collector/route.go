package collector

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc) {
	sources := rg.Group("/sources")
	sources.Use(authRequired)

	sources.POST("/:id/collect", h.CollectSource)
}
