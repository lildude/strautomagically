// Package cache implements a Redis cache.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	redis "github.com/go-redis/redis/v8"
)

type Cache interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, value any) error
	GetJSON(ctx context.Context, key string, value any) error
	SetJSON(ctx context.Context, key string, value any) error
}

type RedisCache struct {
	conn *redis.Client
}

func NewRedisCache(ctx context.Context, addr string) (Cache, error) {
	opt, err := redis.ParseURL(addr)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}
	client := redis.NewClient(opt)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	return &RedisCache{conn: client}, nil
}

// Set stores a value in the cache.
func (rc *RedisCache) Set(ctx context.Context, key string, value any) error {
	return rc.conn.Set(ctx, key, value, 0).Err()
}

// Get retrieves a value from the cache.
func (rc *RedisCache) Get(ctx context.Context, key string) (any, error) {
	value, err := rc.conn.Get(ctx, key).Result()
	if err == nil || errors.Is(err, redis.Nil) {
		return value, nil
	}

	return nil, err
}

// GetJSON retrieves a JSON string and unmarshals it into the given value.
func (rc *RedisCache) GetJSON(ctx context.Context, key string, value any) error {
	v, err := rc.Get(ctx, key)
	if err != nil {
		return err
	}

	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("cache value for %q is not a string: %T", key, v)
	}

	if err := json.Unmarshal([]byte(s), &value); err != nil {
		return fmt.Errorf("unmarshaling cached JSON for %q: %w", key, err)
	}
	return nil
}

// SetJSON stores a struct as a JSON string.
func (rc *RedisCache) SetJSON(ctx context.Context, key string, value any) error {
	t, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling JSON for cache key %q: %w", key, err)
	}
	return rc.Set(ctx, key, string(t))
}
