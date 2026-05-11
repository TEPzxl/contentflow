package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "contentflow",
	})
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	deps := gin.H{
		"postgres": "ok",
		"redis":    "ok",
	}

	if err := h.checkPostgres(ctx); err != nil {
		deps["postgres"] = err.Error()
	}

	if err := h.checkRedis(ctx); err != nil {
		deps["redis"] = err.Error()
	}

	statusCode := http.StatusOK
	status := "ready"

	if deps["postgres"] != "ok" || deps["redis"] != "ok" {
		statusCode = http.StatusServiceUnavailable
		status = "not ready"
	}

	c.JSON(statusCode, gin.H{
		"status":  status,
		"service": "contentflow",
		"deps":    deps,
	})
}

func (h *HealthHandler) checkPostgres(ctx context.Context) error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}

	return sqlDB.PingContext(ctx)
}

func (h *HealthHandler) checkRedis(ctx context.Context) error {
	return h.redis.Ping(ctx).Err()
}
