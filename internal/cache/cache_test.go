package cache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"golang.org/x/oauth2"
)

func TestSetGet(t *testing.T) {
	r := miniredis.RunT(t)
	defer r.Close()
	os.Setenv("REDIS_URL", fmt.Sprintf("redis://%s", r.Addr()))
	cache, err := NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	err = cache.Set("test", "test")
	if err != nil {
		t.Fatal(err)
	}
	value, err := cache.Get("test")
	if err != nil {
		t.Fatal(err)
	}
	if value != "test" {
		t.Fatalf("expected test, got %s", value)
	}
}

func TestSetGetToken(t *testing.T) {
	r := miniredis.RunT(t)
	defer r.Close()
	os.Setenv("REDIS_URL", fmt.Sprintf("redis://%s", r.Addr()))

	token := &oauth2.Token{AccessToken: "test", RefreshToken: "test", TokenType: "test", Expiry: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}
	err := SetToken("test", token)
	if err != nil {
		t.Fatal(err)
	}
	value, err := GetToken("test")
	if err != nil {
		t.Fatal(err)
	}
	if value.AccessToken != token.AccessToken {
		t.Fatalf("expected %v, got %v", token, value)
	}
	if value.RefreshToken != token.RefreshToken {
		t.Fatalf("expected %v, got %v", token, value)
	}
	if value.TokenType != token.TokenType {
		t.Fatalf("expected %v, got %v", token, value)
	}
	if value.Expiry.Unix() != token.Expiry.Unix() {
		t.Fatalf("expected %v, got %v", token, value)
	}
}
