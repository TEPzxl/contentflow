package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tepzxl/contentflow/internal/http/handler"
	"github.com/tepzxl/contentflow/internal/http/middleware"
	"gorm.io/gorm"
)

func NewRouter(log *slog.Logger, db *gorm.DB, redisClient *redis.Client) *gin.Engine {
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(log))
	healthHandler := handler.NewHealthHandler(db, redisClient)

	r.GET("/healthz", healthHandler.Liveness)
	r.GET("/readyz", healthHandler.Readiness)
	return r
}
