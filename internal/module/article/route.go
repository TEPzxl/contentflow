package article

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc) {
	articles := rg.Group("/articles")
	articles.Use(authRequired)

	articles.GET("", h.List)
	articles.GET("/:id", h.Get)
	articles.PATCH("/:id/read", h.MarkRead)
	articles.PATCH("/:id/save", h.Save)
}
