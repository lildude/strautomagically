package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-redis/redis/v8"
	"golang.org/x/oauth2"
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

func GetToken() (*oauth2.Token, error) {
	cache, err := NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		return nil, err
	}

	token := &oauth2.Token{}
	at, err := cache.Get("strava_auth_token")
	if err != nil {
		return nil, err
	}
	if at != "" {
		err = json.Unmarshal([]byte(fmt.Sprint(at)), &token)
		if err != nil {
			return nil, err
		}
	}
	return token, nil
}

func SetToken(token *oauth2.Token) error {
	cache, err := NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		return err
	}

	t, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return cache.Set("strava_auth_token", string(t))
}
