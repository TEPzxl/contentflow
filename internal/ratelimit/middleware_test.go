package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
)

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		limiter    *fakeLimiter
		wantStatus int
		wantCalls  int
	}{
		{
			name: "allowed",
			limiter: &fakeLimiter{
				result: Result{
					Allowed:   true,
					Limit:     2,
					Remaining: 1,
				},
			},
			wantStatus: http.StatusOK,
			wantCalls:  1,
		},
		{
			name: "blocked",
			limiter: &fakeLimiter{
				result: Result{
					Allowed:    false,
					Limit:      2,
					Remaining:  0,
					RetryAfter: time.Minute,
				},
			},
			wantStatus: http.StatusTooManyRequests,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(Middleware(tt.limiter, Config{
				Limit:  2,
				Window: time.Minute,
				KeyFunc: func(c *gin.Context) string {
					return "ratelimit:test"
				},
			}))
			r.GET("/ping", func(c *gin.Context) {
				c.String(http.StatusOK, "pong")
			})

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.limiter.calls != tt.wantCalls {
				t.Fatalf("limiter calls = %d, want %d", tt.limiter.calls, tt.wantCalls)
			}
		})
	}
}

func TestUserIDPathKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/sources/:id/collect", func(c *gin.Context) {
		ctx := requestctx.WithUserID(c.Request.Context(), 100)
		c.Request = c.Request.WithContext(ctx)

		got := UserIDPathKey("ratelimit:collect", "id")(c)
		if got != "ratelimit:collect:user:100:id:42" {
			t.Fatalf("key = %q", got)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/sources/42/collect", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

type fakeLimiter struct {
	result Result
	err    error
	calls  int
}

func (f *fakeLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error) {
	f.calls++
	if f.err != nil {
		return Result{}, f.err
	}
	return f.result, nil
}
