package cache

import (
	"fmt"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestSetGet(t *testing.T) {
	r := miniredis.RunT(t)
	defer r.Close()
	os.Setenv("REDIS_URL", fmt.Sprintf("redis://%s", r.Addr()))
	cache, err := NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		t.Error(err)
	}
	err = cache.Set("test", "test")
	if err != nil {
		t.Error(err)
	}
	value, err := cache.Get("test")
	if err != nil {
		t.Error(err)
	}
	if value != "test" {
		t.Errorf("expected test, got %s", value)
	}
}
