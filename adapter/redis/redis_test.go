package redis

import (
	"context"
	"reflect"
	"testing"
	"time"

	cache "github.com/cludden/http-cache"
	redisCache "github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
)

var a cache.Adapter = NewAdapter(redisCache.New(&redisCache.Options{
	Redis: redis.NewClient(&redis.Options{
		Addr: ":6379",
	}),
}))

func TestSet(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		response []byte
	}{
		{
			"sets a response cache",
			"https://example.com/foo",
			cache.Response{
				Value:      []byte("value 1"),
				Expiration: time.Now().Add(1 * time.Minute),
			}.Bytes(),
		},
		{
			"sets a response cache",
			"https://example.com/bar",
			cache.Response{
				Value:      []byte("value 2"),
				Expiration: time.Now().Add(1 * time.Minute),
			}.Bytes(),
		},
		{
			"sets a response cache",
			"https://example.com/baz",
			cache.Response{
				Value:      []byte("value 3"),
				Expiration: time.Now().Add(1 * time.Minute),
			}.Bytes(),
		},
	}
	for _, tt := range tests {
		// t.Run(tt.name, func(t *testing.T) {
		// 	a.Set(context.Background(), tt.key, tt.response, time.Now().Add(1*time.Minute))
		// })
		a.Set(context.Background(), tt.key, tt.response, time.Now().Add(1*time.Minute))
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want []byte
		ok   bool
	}{
		{
			"returns right response",
			"https://example.com/foo",
			[]byte("value 1"),
			true,
		},
		{
			"returns right response",
			"https://example.com/bar",
			[]byte("value 2"),
			true,
		},
		{
			"key does not exist",
			"https://example.com/qux",
			nil,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, ok := a.Get(context.Background(), tt.key)
			if ok != tt.ok {
				t.Errorf("memory.Get() ok = %v, tt.ok %v", ok, tt.ok)
				return
			}
			got := cache.BytesToResponse(b).Value
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("memory.Get() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

func TestRelease(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			"removes cached response from store",
			"https://example.com/foo",
		},
		{
			"removes cached response from store",
			"https://example.com/bar",
		},
		{
			"removes cached response from store",
			"https://example.com/baz",
		},
		{
			"key does not exist",
			"https://example.com/qux",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a.Release(context.Background(), tt.key)
			if _, ok := a.Get(context.Background(), tt.key); ok {
				t.Errorf("memory.Release() error; key %v should not be found", tt.key)
			}
		})
	}
}
