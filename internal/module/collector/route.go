package collector

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc, collectMiddlewares ...gin.HandlerFunc) {
	sources := rg.Group("/sources")
	sources.Use(authRequired)

	sources.POST("/:id/collect", append(collectMiddlewares, h.CollectSource)...)
	RegisterCollectionRunRoutes(rg, h, authRequired)
}

func RegisterCollectionRunRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc) {
	sources := rg.Group("/sources")
	sources.Use(authRequired)

	sources.GET("/:id/collection-runs", h.ListCollectionRuns)

	rg.GET("/collection-runs/:id", authRequired, h.GetCollectionRun)
}

func RegisterAsyncRoutes(rg *gin.RouterGroup, h *AsyncHandler, authRequired gin.HandlerFunc, collectMiddlewares ...gin.HandlerFunc) {
	sources := rg.Group("/sources")
	sources.Use(authRequired)

	sources.POST("/:id/collect", append(collectMiddlewares, h.RequestCollection)...)
}
