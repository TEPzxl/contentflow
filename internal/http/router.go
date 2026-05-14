package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	apidocs "github.com/tepzxl/contentflow/api"
	"github.com/tepzxl/contentflow/internal/http/handler"
	"github.com/tepzxl/contentflow/internal/http/middleware"
	"github.com/tepzxl/contentflow/internal/observability"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"gorm.io/gorm"
)

type RegisterRoutesFunc func(api *gin.RouterGroup)

type routerOptions struct {
	metrics            *observability.Metrics
	tracingServiceName string
}

type RouterOption func(*routerOptions)

func WithMetrics(metrics *observability.Metrics) RouterOption {
	return func(opts *routerOptions) {
		opts.metrics = metrics
	}
}

func WithTracing(serviceName string) RouterOption {
	return func(opts *routerOptions) {
		opts.tracingServiceName = serviceName
	}
}

func NewRouter(log *slog.Logger, db *gorm.DB, redisClient *redis.Client, registerRoutes RegisterRoutesFunc, opts ...RouterOption) *gin.Engine {
	options := routerOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	if options.tracingServiceName != "" {
		r.Use(otelgin.Middleware(options.tracingServiceName))
	}
	if options.metrics != nil {
		r.Use(options.metrics.HTTPMiddleware())
	}
	r.Use(middleware.RequestLogger(log))
	healthHandler := handler.NewHealthHandler(db, redisClient)

	api := r.Group("/api/v1")
	if registerRoutes != nil {
		registerRoutes(api)
	}

	r.GET("/healthz", healthHandler.Liveness)
	r.GET("/readyz", healthHandler.Readiness)
	if options.metrics != nil {
		r.GET("/metrics", gin.WrapH(options.metrics.Handler()))
	}
	apidocs.RegisterRoutes(r)
	return r
}
