/*
MIT License

Copyright (c) 2018 Victor Springer

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package redis

import (
	"context"
	"time"

	cache "github.com/cludden/http-cache"
	redis "github.com/go-redis/cache/v8"
)

// Adapter is the memory adapter data structure.
type Adapter struct {
	store *redis.Cache
}

// Get implements the cache Adapter interface Get method.
func (a *Adapter) Get(ctx context.Context, key string) ([]byte, bool) {
	var c []byte
	if err := a.store.Get(ctx, key, &c); err == nil {
		return c, true
	}

	return nil, false
}

// Set implements the cache Adapter interface Set method.
func (a *Adapter) Set(ctx context.Context, key string, response []byte, expiration time.Time) {
	a.store.Set(&redis.Item{
		Key:   key,
		Value: response,
		TTL:   time.Until(expiration),
	})
}

// Release implements the cache Adapter interface Release method.
func (a *Adapter) Release(ctx context.Context, key string) {
	a.store.Delete(ctx, key)
}

// NewAdapter initializes Redis adapter.
func NewAdapter(c *redis.Cache) cache.Adapter {
	return &Adapter{
		store: c,
	}
}
