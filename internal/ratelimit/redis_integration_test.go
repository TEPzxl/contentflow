//go:build integration

package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRedisLimiter_Allow(t *testing.T) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminate redis container: %v", err)
		}
	}()

	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("get redis endpoint: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: endpoint})
	defer client.Close()

	limiter := NewRedisLimiter(client)

	first, err := limiter.Allow(ctx, "ratelimit:test", 2, time.Minute)
	if err != nil {
		t.Fatalf("first Allow() error = %v", err)
	}
	if !first.Allowed || first.Remaining != 1 {
		t.Fatalf("first result = %#v", first)
	}

	second, err := limiter.Allow(ctx, "ratelimit:test", 2, time.Minute)
	if err != nil {
		t.Fatalf("second Allow() error = %v", err)
	}
	if !second.Allowed || second.Remaining != 0 {
		t.Fatalf("second result = %#v", second)
	}

	third, err := limiter.Allow(ctx, "ratelimit:test", 2, time.Minute)
	if err != nil {
		t.Fatalf("third Allow() error = %v", err)
	}
	if third.Allowed {
		t.Fatalf("third allowed = true, want false")
	}
	if third.RetryAfter <= 0 {
		t.Fatalf("third RetryAfter = %v, want > 0", third.RetryAfter)
	}
}
