package cache

import (
	"fmt"
	"os"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestSetGet(t *testing.T) {
	r := miniredis.RunT(t)
	defer r.Close()
	t.Setenv("REDIS_URL", fmt.Sprintf("redis://%s", r.Addr()))
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

func TestSetGetJSON(t *testing.T) {
	r := miniredis.RunT(t)
	defer r.Close()
	t.Setenv("REDIS_URL", fmt.Sprintf("redis://%s", r.Addr()))
	cache, err := NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		t.Error(err)
	}
	// test struct that will be marshalled to JSON
	type Test struct {
		Name string
		Age  int
	}
	test := Test{
		Name: "jsontest",
		Age:  10,
	}
	err = cache.SetJSON("jsontest", test)
	if err != nil {
		t.Error(err)
	}
	// Confirm the value is stored in the cache as a JSON string
	js, err := cache.Get("jsontest")
	if err != nil {
		t.Error(err)
	}
	if js != `{"Name":"jsontest","Age":10}` {
		t.Errorf("expected `{\"Name\":\"jsontest\",\"Age\":10}`, got %s", js)
	}

	// Confirm the value is unmarshalled into the given interface
	var test2 Test
	err = cache.GetJSON("jsontest", &test2)
	if err != nil {
		t.Error(err)
	}
	if test2.Name != "jsontest" || test2.Age != 10 {
		t.Errorf("expected {\"Name\":\"jsontest\",\"Age\":10}, got %v", test2)
	}
}
