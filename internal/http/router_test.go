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

func TestNewRouter_AddsSecurityHeaders(t *testing.T) {
	router := contenthttp.NewRouter(
		slog.Default(),
		nil,
		nil,
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assertHeader(t, w, "X-Content-Type-Options", "nosniff")
	assertHeader(t, w, "X-Frame-Options", "DENY")
	assertHeader(t, w, "Referrer-Policy", "strict-origin-when-cross-origin")
	assertHeader(t, w, "Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
	assertHeader(t, w, "Permissions-Policy", "camera=(), microphone=(), geolocation=()")
}

func TestNewRouter_AllowsRedocAssets(t *testing.T) {
	router := contenthttp.NewRouter(
		slog.Default(),
		nil,
		nil,
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	assertHeader(t, w, "Content-Security-Policy", "default-src 'self'; script-src 'self' https://cdn.redoc.ly; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
}

func TestNewRouter_AllowsConfiguredCORSPreflight(t *testing.T) {
	router := contenthttp.NewRouter(
		slog.Default(),
		nil,
		nil,
		nil,
		contenthttp.WithCORSAllowedOrigins([]string{"http://localhost:3001"}),
	)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3001")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type, authorization")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	assertHeader(t, w, "Access-Control-Allow-Origin", "http://localhost:3001")
	assertHeader(t, w, "Access-Control-Allow-Credentials", "true")
	assertHeader(t, w, "Access-Control-Allow-Headers", "Authorization, Content-Type")
	assertHeader(t, w, "Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
}

func TestNewRouter_DoesNotAllowUnknownCORSOrigin(t *testing.T) {
	router := contenthttp.NewRouter(
		slog.Default(),
		nil,
		nil,
		nil,
		contenthttp.WithCORSAllowedOrigins([]string{"http://localhost:3001"}),
	)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func assertHeader(t *testing.T, w *httptest.ResponseRecorder, name string, want string) {
	t.Helper()
	if got := w.Header().Get(name); got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}
