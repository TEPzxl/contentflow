package http

import (
	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/handler"
)

func NewRouter() *gin.Engine {
	r := gin.New()

	r.Use(gin.Recovery())
	healthHandler := handler.NewHealthHandler()

	r.GET("/healthz", healthHandler.Check)
	return r
}
