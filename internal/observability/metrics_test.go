package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/tepzxl/contentflow/internal/observability"
)

func TestMetrics_HTTPMiddlewareRecordsRequestTotal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := prometheus.NewRegistry()
	metrics, err := observability.NewMetrics(registry)
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	r := gin.New()
	r.Use(metrics.HTTPMiddleware())
	r.GET("/ping/:id", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping/123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	expected := `
# HELP contentflow_http_requests_total Total HTTP requests handled by Contentflow.
# TYPE contentflow_http_requests_total counter
contentflow_http_requests_total{method="GET",path="/ping/:id",status="201"} 1
`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(expected), "contentflow_http_requests_total"); err != nil {
		t.Fatalf("unexpected metrics output: %v", err)
	}
}
