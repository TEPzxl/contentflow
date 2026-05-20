package collectionjob

import "github.com/gin-gonic/gin"

func RegisterDLQRoutes(rg *gin.RouterGroup, h *DLQHandler, authRequired gin.HandlerFunc) {
	dlq := rg.Group("/collection-dlq")
	dlq.Use(authRequired)

	dlq.GET("", h.List)
	dlq.POST("/:id/replay", h.Replay)
	dlq.POST("/:id/handled", h.MarkHandled)
}
