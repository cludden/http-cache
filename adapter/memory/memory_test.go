package memory

import (
	"reflect"
	"sync"
	"testing"
	"time"

	cache "github.com/cludden/http-cache"
)

func TestGet(t *testing.T) {
	a := &Adapter{
		sync.RWMutex{},
		2,
		LRU,
		map[string][]byte{
			"https://example.com/foo": cache.Response{
				Value:      []byte("value 1"),
				Expiration: time.Now(),
				LastAccess: time.Now(),
				Frequency:  1,
			}.Bytes(),
		},
	}

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
			"not found",
			"https://example.com/bar",
			nil,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, ok := a.Get(tt.key)
			if ok != tt.ok {
				t.Errorf("memory.Get() ok = %v, tt.ok %v", ok, tt.ok)
				return
			}
			got := cache.BytesToResponse(b).Value
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("memory.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSet(t *testing.T) {
	a := &Adapter{
		sync.RWMutex{},
		2,
		LRU,
		make(map[string][]byte),
	}

	tests := []struct {
		name     string
		key      string
		response cache.Response
	}{
		{
			"sets a response cache",
			"https://example.com/foo",
			cache.Response{
				Value:      []byte("value 1"),
				Expiration: time.Now().Add(1 * time.Minute),
			},
		},
		{
			"sets a response cache",
			"https://example.com/bar",
			cache.Response{
				Value:      []byte("value 2"),
				Expiration: time.Now().Add(1 * time.Minute),
			},
		},
		{
			"sets a response cache",
			"https://example.com/baz",
			cache.Response{
				Value:      []byte("value 3"),
				Expiration: time.Now().Add(1 * time.Minute),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a.Set(tt.key, tt.response.Bytes(), tt.response.Expiration)
			if cache.BytesToResponse(a.store[tt.key]).Value == nil {
				t.Errorf(
					"memory.Set() error = store[%v] response is not %s", tt.key, tt.response.Value,
				)
			}
		})
	}
}

func TestRelease(t *testing.T) {
	a := &Adapter{
		sync.RWMutex{},
		2,
		LRU,
		map[string][]byte{
			"https://example.com/foo": cache.Response{
				Expiration: time.Now().Add(1 * time.Minute),
				Value:      []byte("value 1"),
			}.Bytes(),
			"https://example.com/bar": cache.Response{
				Expiration: time.Now(),
				Value:      []byte("value 2"),
			}.Bytes(),
			"https://example.com/baz": cache.Response{
				Expiration: time.Now(),
				Value:      []byte("value 3"),
			}.Bytes(),
		},
	}

	tests := []struct {
		name        string
		key         string
		storeLength int
		wantErr     bool
	}{
		{
			"removes cached response from store",
			"https://example.com/foo",
			2,
			false,
		},
		{
			"removes cached response from store",
			"https://example.com/bar",
			1,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a.Release(tt.key)
			if len(a.store) > tt.storeLength {
				t.Errorf("memory.Release() error; store length = %v, want 0", len(a.store))
			}
		})
	}
}

func TestNewAdapter(t *testing.T) {
	tests := []struct {
		name    string
		opts    []AdapterOptions
		want    cache.Adapter
		wantErr bool
	}{
		{
			"returns new Adapter",
			[]AdapterOptions{
				AdapterWithCapacity(4),
				AdapterWithAlgorithm(LRU),
			},
			&Adapter{
				sync.RWMutex{},
				4,
				LRU,
				make(map[string][]byte),
			},
			false,
		},
		{
			"returns error",
			[]AdapterOptions{
				AdapterWithAlgorithm(LRU),
			},
			nil,
			true,
		},
		{
			"returns error",
			[]AdapterOptions{
				AdapterWithCapacity(4),
			},
			nil,
			true,
		},
		{
			"returns error",
			[]AdapterOptions{
				AdapterWithCapacity(1),
			},
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAdapter(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAdapter() = %v, want %v", got, tt.want)
			}
		})
	}
}
