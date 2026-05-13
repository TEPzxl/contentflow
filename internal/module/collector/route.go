package collector

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc, collectMiddlewares ...gin.HandlerFunc) {
	sources := rg.Group("/sources")
	sources.Use(authRequired)

	sources.POST("/:id/collect", append(collectMiddlewares, h.CollectSource)...)
}

func RegisterAsyncRoutes(rg *gin.RouterGroup, h *AsyncHandler, authRequired gin.HandlerFunc, collectMiddlewares ...gin.HandlerFunc) {
	sources := rg.Group("/sources")
	sources.Use(authRequired)

	sources.POST("/:id/collect", append(collectMiddlewares, h.RequestCollection)...)
}
