package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
)

func RequestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("duration", duration),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
		}

		if requestID, ok := requestctx.RequestID(c.Request.Context()); ok {
			attrs = append(attrs, slog.String("request_id", requestID))
		}

		if userID, ok := requestctx.UserID(c.Request.Context()); ok {
			attrs = append(attrs, slog.Int64("user_id", userID))
		}

		log.Info("http request completed", attrs...)
	}
}
