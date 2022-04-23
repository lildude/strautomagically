package cache

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type Cache interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}) error
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

func (rc *RedisCache) Set(key string, value interface{}) error {
	return rc.conn.Set(rc.ctx, key, value, 0).Err()
}

func (rc *RedisCache) Get(key string) (interface{}, error) {
	if value, err := rc.conn.Get(rc.ctx, key).Result(); err == nil || err == redis.Nil {
		return value, nil
	} else {
		return nil, err
	}
}
