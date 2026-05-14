package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/middleware"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
)

func TestRequestID_PreservesIncomingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/ping", func(c *gin.Context) {
		requestID, ok := requestctx.RequestID(c.Request.Context())
		if !ok {
			t.Fatal("request id missing from context")
		}
		c.String(http.StatusOK, requestID)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "req-123")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("X-Request-ID"); got != "req-123" {
		t.Fatalf("response request id = %q, want req-123", got)
	}
	if got := w.Body.String(); got != "req-123" {
		t.Fatalf("context request id = %q, want req-123", got)
	}
}

func TestRequestID_GeneratesMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/ping", func(c *gin.Context) {
		requestID, ok := requestctx.RequestID(c.Request.Context())
		if !ok {
			t.Fatal("request id missing from context")
		}
		c.String(http.StatusOK, requestID)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatal("response request id is empty")
	}
	if got := w.Body.String(); got != requestID {
		t.Fatalf("context request id = %q, want generated response header %q", got, requestID)
	}
}
