package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	authGroup := rg.Group("/auth")

	authGroup.POST("/register", h.Register)
	authGroup.POST("/login", h.Login)
}
