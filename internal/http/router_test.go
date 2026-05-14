package http_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	contenthttp "github.com/tepzxl/contentflow/internal/http"
	"github.com/tepzxl/contentflow/internal/observability"
)

func TestNewRouter_RegistersMetricsEndpoint(t *testing.T) {
	metrics, err := observability.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}

	router := contenthttp.NewRouter(
		slog.Default(),
		nil,
		nil,
		nil,
		contenthttp.WithMetrics(metrics),
	)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
