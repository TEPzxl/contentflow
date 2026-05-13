package source

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc) {
	sources := rg.Group("/sources")
	sources.Use(authRequired)

	sources.POST("", h.Create)
	sources.GET("", h.List)
	sources.GET("/:id", h.Get)
	sources.PUT("/:id", h.Update)
	sources.DELETE("/:id", h.Delete)
}
