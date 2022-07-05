package cache

import (
	"context"
	"encoding/json"

	"github.com/go-redis/redis/v8"
)

type Cache interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}) error
	GetJSON(key string, v interface{}) error
	SetJSON(key string, value interface{}) error
}

type RedisCache struct {
	conn *redis.Client
	ctx  context.Context
}

func NewRedisCache(addr string) (Cache, error) {
	opt, err := redis.ParseURL(addr)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	client := redis.NewClient(opt)

	_, err = client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return &RedisCache{
		conn: client,
		ctx:  ctx,
	}, nil
}

// Set stores a value in the cache
func (rc *RedisCache) Set(key string, value interface{}) error {
	return rc.conn.Set(rc.ctx, key, value, 0).Err()
}

// Get retrieves a value from the cache
func (rc *RedisCache) Get(key string) (interface{}, error) {
	if value, err := rc.conn.Get(rc.ctx, key).Result(); err == nil || err == redis.Nil {
		return value, nil
	} else {
		return nil, err
	}
}

// GetJSON retrieves a JSON string and unmarshals it into the given interface
func (rc *RedisCache) GetJSON(key string, value interface{}) error {
	v, err := rc.Get(key)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(v.(string)), &value); err != nil {
		return err
	}
	return nil
}

// SetJSON stores a struct as a JSON string
func (rc *RedisCache) SetJSON(key string, value interface{}) error {
	t, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return rc.Set(key, string(t))
}
