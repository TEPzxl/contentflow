package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisCollectionLock struct {
	client *redis.Client
	prefix string
}

func NewRedisCollectionLock(client *redis.Client) *RedisCollectionLock {
	return &RedisCollectionLock{
		client: client,
		prefix: "collection:lock:source",
	}
}

func (l *RedisCollectionLock) Acquire(ctx context.Context, sourceID int64, ttl time.Duration) (CollectionLockReleaseFunc, bool, error) {
	if l == nil || l.client == nil {
		return nil, true, nil
	}
	if ttl <= 0 {
		ttl = defaultCollectionLockTTL
	}

	key := fmt.Sprintf("%s:%d", l.prefix, sourceID)
	value := uuid.NewString()

	acquired, err := l.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, nil
	}

	return func(ctx context.Context) error {
		const script = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`
		return l.client.Eval(ctx, script, []string{key}, value).Err()
	}, true, nil
}
