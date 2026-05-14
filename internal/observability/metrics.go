package observability

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tepzxl/contentflow/internal/module/collectionjob"
	"github.com/tepzxl/contentflow/internal/module/collector"
	"gorm.io/gorm"
)

const dbStartKey = "contentflow_observability_db_start"

type Metrics struct {
	gatherer prometheus.Gatherer

	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	collectionRunsTotal  *prometheus.CounterVec
	collectionItemsTotal *prometheus.CounterVec
	kafkaJobsTotal       *prometheus.CounterVec
	dbQueriesTotal       *prometheus.CounterVec
	dbQueryDuration      *prometheus.HistogramVec
}

func NewMetrics(registry *prometheus.Registry) (*Metrics, error) {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	return newMetrics(registry, registry)
}

func NewDefaultMetrics() (*Metrics, error) {
	return newMetrics(prometheus.DefaultRegisterer, prometheus.DefaultGatherer)
}

func newMetrics(registerer prometheus.Registerer, gatherer prometheus.Gatherer) (*Metrics, error) {
	m := &Metrics{
		gatherer: gatherer,
		httpRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "contentflow_http_requests_total",
			Help: "Total HTTP requests handled by Contentflow.",
		}, []string{"method", "path", "status"}),
		httpRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "contentflow_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),
		collectionRunsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "contentflow_collection_runs_total",
			Help: "Total collection runs grouped by source type and status.",
		}, []string{"source_type", "status"}),
		collectionItemsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "contentflow_collection_items_total",
			Help: "Total collection item counts grouped by source type and count kind.",
		}, []string{"source_type", "kind"}),
		kafkaJobsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "contentflow_kafka_jobs_total",
			Help: "Total Kafka collection job events grouped by topic and status.",
		}, []string{"topic", "status"}),
		dbQueriesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "contentflow_db_queries_total",
			Help: "Total GORM operations grouped by operation and status.",
		}, []string{"operation", "status"}),
		dbQueryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "contentflow_db_query_duration_seconds",
			Help:    "GORM operation duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation", "status"}),
	}

	collectors := []prometheus.Collector{
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.collectionRunsTotal,
		m.collectionItemsTotal,
		m.kafkaJobsTotal,
		m.dbQueriesTotal,
		m.dbQueryDuration,
	}

	for _, c := range collectors {
		if err := registerer.Register(c); err != nil {
			return nil, fmt.Errorf("register metric: %w", err)
		}
	}

	return m, nil
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.gatherer, promhttp.HandlerOpts{})
}

func (m *Metrics) HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		duration := time.Since(start).Seconds()

		m.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		m.httpRequestDuration.WithLabelValues(method, path, status).Observe(duration)
	}
}

func (m *Metrics) ObserveCollection(_ context.Context, observation collector.CollectionObservation) {
	sourceType := observation.SourceType
	if sourceType == "" {
		sourceType = "unknown"
	}

	m.collectionRunsTotal.WithLabelValues(sourceType, observation.Status).Inc()
	m.collectionItemsTotal.WithLabelValues(sourceType, "fetched").Add(float64(observation.FetchedCount))
	m.collectionItemsTotal.WithLabelValues(sourceType, "inserted").Add(float64(observation.InsertedCount))
	m.collectionItemsTotal.WithLabelValues(sourceType, "duplicated").Add(float64(observation.DuplicatedCount))
}

func (m *Metrics) ObserveKafkaJob(_ context.Context, topic string, status string) {
	m.kafkaJobsTotal.WithLabelValues(topic, status).Inc()
}

func (m *Metrics) ObserveJob(ctx context.Context, observation collectionjob.JobObservation) {
	m.ObserveKafkaJob(ctx, observation.Topic, observation.Status)
}

func (m *Metrics) RegisterGormCallbacks(db *gorm.DB) error {
	if err := db.Callback().Query().Before("gorm:query").Register("contentflow:metrics:before:query", beforeDB); err != nil {
		return fmt.Errorf("register query before callback: %w", err)
	}
	if err := db.Callback().Query().After("gorm:query").Register("contentflow:metrics:after:query", m.afterDB("query")); err != nil {
		return fmt.Errorf("register query after callback: %w", err)
	}
	if err := db.Callback().Create().Before("gorm:create").Register("contentflow:metrics:before:create", beforeDB); err != nil {
		return fmt.Errorf("register create before callback: %w", err)
	}
	if err := db.Callback().Create().After("gorm:create").Register("contentflow:metrics:after:create", m.afterDB("create")); err != nil {
		return fmt.Errorf("register create after callback: %w", err)
	}
	if err := db.Callback().Update().Before("gorm:update").Register("contentflow:metrics:before:update", beforeDB); err != nil {
		return fmt.Errorf("register update before callback: %w", err)
	}
	if err := db.Callback().Update().After("gorm:update").Register("contentflow:metrics:after:update", m.afterDB("update")); err != nil {
		return fmt.Errorf("register update after callback: %w", err)
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("contentflow:metrics:before:delete", beforeDB); err != nil {
		return fmt.Errorf("register delete before callback: %w", err)
	}
	if err := db.Callback().Delete().After("gorm:delete").Register("contentflow:metrics:after:delete", m.afterDB("delete")); err != nil {
		return fmt.Errorf("register delete after callback: %w", err)
	}
	if err := db.Callback().Raw().Before("gorm:raw").Register("contentflow:metrics:before:raw", beforeDB); err != nil {
		return fmt.Errorf("register raw before callback: %w", err)
	}
	if err := db.Callback().Raw().After("gorm:raw").Register("contentflow:metrics:after:raw", m.afterDB("raw")); err != nil {
		return fmt.Errorf("register raw after callback: %w", err)
	}
	if err := db.Callback().Row().Before("gorm:row").Register("contentflow:metrics:before:row", beforeDB); err != nil {
		return fmt.Errorf("register row before callback: %w", err)
	}
	if err := db.Callback().Row().After("gorm:row").Register("contentflow:metrics:after:row", m.afterDB("row")); err != nil {
		return fmt.Errorf("register row after callback: %w", err)
	}

	return nil
}

func beforeDB(db *gorm.DB) {
	db.InstanceSet(dbStartKey, time.Now())
}

func (m *Metrics) afterDB(operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		value, ok := db.InstanceGet(dbStartKey)
		if !ok {
			return
		}

		start, ok := value.(time.Time)
		if !ok {
			return
		}

		status := "success"
		if db.Error != nil {
			status = "error"
		}

		m.dbQueriesTotal.WithLabelValues(operation, status).Inc()
		m.dbQueryDuration.WithLabelValues(operation, status).Observe(time.Since(start).Seconds())
	}
}
