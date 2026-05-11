package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/handler"
	"github.com/tepzxl/contentflow/internal/http/middleware"
)

func NewRouter(log *slog.Logger) *gin.Engine {
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(log))
	healthHandler := handler.NewHealthHandler()

	r.GET("/healthz", healthHandler.Check)
	return r
}
