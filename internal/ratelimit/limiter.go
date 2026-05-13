package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
)

type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
	ResetAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error)
}

type Config struct {
	Limit   int
	Window  time.Duration
	KeyFunc func(c *gin.Context) string
}

func Middleware(limiter Limiter, cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil || cfg.Limit <= 0 || cfg.Window <= 0 || cfg.KeyFunc == nil {
			c.Next()
			return
		}

		key := cfg.KeyFunc(c)
		if key == "" {
			c.Next()
			return
		}

		result, err := limiter.Allow(c.Request.Context(), key, cfg.Limit, cfg.Window)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "rate_limit_error", "rate limit error")
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		if result.RetryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
		}

		if !result.Allowed {
			response.Error(c, http.StatusTooManyRequests, "rate_limited", "too many requests")
			c.Abort()
			return
		}

		c.Next()
	}
}

func ClientIPKey(prefix string) func(c *gin.Context) string {
	return func(c *gin.Context) string {
		return fmt.Sprintf("%s:ip:%s", prefix, c.ClientIP())
	}
}

func UserIDPathKey(prefix string, paramName string) func(c *gin.Context) string {
	return func(c *gin.Context) string {
		userID, ok := requestctx.UserID(c.Request.Context())
		if !ok {
			return ""
		}

		return fmt.Sprintf("%s:user:%d:id:%s", prefix, userID, c.Param(paramName))
	}
}

type RedisLimiter struct {
	client redis.Cmdable
}

func NewRedisLimiter(client redis.Cmdable) *RedisLimiter {
	return &RedisLimiter{client: client}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error) {
	values, err := redisRateLimitScript.Run(ctx, l.client, []string{key}, limit, int(window.Milliseconds())).Slice()
	if err != nil {
		return Result{}, fmt.Errorf("run rate limit script: %w", err)
	}

	allowed := toInt64(values[0]) == 1
	count := int(toInt64(values[1]))
	ttl := time.Duration(toInt64(values[2])) * time.Millisecond
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	result := Result{
		Allowed:    allowed,
		Limit:      limit,
		Remaining:  remaining,
		ResetAfter: ttl,
	}
	if !allowed {
		result.RetryAfter = ttl
	}

	return result, nil
}

func toInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

var redisRateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
end

local ttl = redis.call("PTTL", KEYS[1])
if ttl < 0 then
  ttl = tonumber(ARGV[2])
  redis.call("PEXPIRE", KEYS[1], ttl)
end

if current > tonumber(ARGV[1]) then
  return {0, current, ttl}
end

return {1, current, ttl}
`)
