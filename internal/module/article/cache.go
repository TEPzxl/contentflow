package article

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisListCache struct {
	client redis.Cmdable
	prefix string
}

func NewRedisListCache(client redis.Cmdable, prefix string) *RedisListCache {
	if prefix == "" {
		prefix = "cache:articles"
	}
	return &RedisListCache{
		client: client,
		prefix: prefix,
	}
}

func (c *RedisListCache) GetList(ctx context.Context, req ListArticlesRequest) (*ListArticlesResponse, bool, error) {
	data, err := c.client.Get(ctx, c.listKey(req)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("get article list cache: %w", err)
	}

	var resp ListArticlesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false, fmt.Errorf("unmarshal article list cache: %w", err)
	}

	return &resp, true, nil
}

func (c *RedisListCache) SetList(ctx context.Context, req ListArticlesRequest, resp *ListArticlesResponse, ttl time.Duration) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal article list cache: %w", err)
	}

	if err := c.client.Set(ctx, c.listKey(req), data, ttl).Err(); err != nil {
		return fmt.Errorf("set article list cache: %w", err)
	}

	return nil
}

func (c *RedisListCache) DeleteUser(ctx context.Context, userID int64) error {
	pattern := fmt.Sprintf("%s:user:%d:*", c.prefix, userID)
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("delete article list cache: %w", err)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("scan article list cache: %w", err)
	}
	return nil
}

func (c *RedisListCache) listKey(req ListArticlesRequest) string {
	return fmt.Sprintf(
		"%s:user:%d:source:%d:q:%s:read:%s:saved:%s:limit:%d:offset:%d",
		c.prefix,
		req.UserID,
		req.SourceID,
		req.Query,
		boolKey(req.IsRead),
		boolKey(req.IsSaved),
		req.Limit,
		req.Offset,
	)
}

func boolKey(value *bool) string {
	if value == nil {
		return "any"
	}
	if *value {
		return "true"
	}
	return "false"
}
