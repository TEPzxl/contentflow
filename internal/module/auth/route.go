package auth

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authRequired gin.HandlerFunc) {
	authGroup := rg.Group("/auth")

	authGroup.POST("/register", h.Register)
	authGroup.POST("/login", h.Login)
	authGroup.POST("/refresh", h.Refresh)
	authGroup.POST("/logout", h.Logout)

	protectedGroup := rg.Group("")
	protectedGroup.Use(authRequired)
	protectedGroup.GET("/me", h.Me)
}
