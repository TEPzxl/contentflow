package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tepzxl/contentflow/internal/http/handler"
	"github.com/tepzxl/contentflow/internal/http/middleware"
	"gorm.io/gorm"
)

type RegisterRoutesFunc func(api *gin.RouterGroup)

func NewRouter(log *slog.Logger, db *gorm.DB, redisClient *redis.Client, registerRoutes RegisterRoutesFunc) *gin.Engine {
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(log))
	healthHandler := handler.NewHealthHandler(db, redisClient)

	api := r.Group("/api/v1")
	if registerRoutes != nil {
		registerRoutes(api)
	}

	r.GET("/healthz", healthHandler.Liveness)
	r.GET("/readyz", healthHandler.Readiness)
	return r
}
