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

package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Adapter interface for HTTP cache middleware client.
type Adapter interface {
	// Get retrieves the cached response by a given key. It also
	// returns true or false, whether it exists or not.
	Get(context.Context, string) ([]byte, bool)

	// Set caches a response for a given key until an expiration date.
	Set(context.Context, string, []byte, time.Time)

	// Release frees cache for a given key.
	Release(context.Context, string)
}

// =============================================================================

// Response is the cached response data structure.
type Response struct {
	// Value is the cached response value.
	Value []byte

	// Header is the cached response header.
	Header http.Header

	// Expiration is the cached response expiration date.
	Expiration time.Time

	// LastAccess is the last date a cached response was accessed.
	// Used by LRU and MRU algorithms.
	LastAccess time.Time

	// Frequency is the count of times a cached response is accessed.
	// Used for LFU and MFU algorithms.
	Frequency int
}

// BytesToResponse converts bytes array into Response data structure.
func BytesToResponse(b []byte) Response {
	var r Response
	dec := gob.NewDecoder(bytes.NewReader(b))
	dec.Decode(&r)

	return r
}

// Bytes converts Response data structure into bytes array.
func (r Response) Bytes() []byte {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	enc.Encode(&r)

	return b.Bytes()
}

// =============================================================================

// ClientOption is used to set Client settings.
type ClientOption func(c *Client) error

// WithAdapter sets the adapter type for the HTTP cache
// middleware client.
func WithAdapter(a Adapter) ClientOption {
	return func(c *Client) error {
		c.adapter = a
		return nil
	}
}

// WithCacheable overrides the default cachable function
func WithCacheable(fn func(*http.Request) bool) ClientOption {
	return func(c *Client) error {
		if fn == nil {
			return fmt.Errorf("cacheable function can not be nil")
		}
		c.cacheableFn = fn
		return nil
	}
}

// WithKey configues the key generation function
func WithKey(fn func(*http.Request) (string, error)) ClientOption {
	return func(c *Client) error {
		if fn == nil {
			return fmt.Errorf("key function can not be nil")
		}
		c.keygenFn = fn
		return nil
	}
}

// WithRefreshKey sets the parameter key used to free a request
// cached response. Optional setting.
func WithRefreshKey(refreshKey string) ClientOption {
	return func(c *Client) error {
		c.refreshKey = refreshKey
		return nil
	}
}

// WithTTL sets how long each response is going to be cached.
func WithTTL(ttl time.Duration) ClientOption {
	return func(c *Client) error {
		if int64(ttl) < 1 {
			return fmt.Errorf("cache client ttl %v is invalid", ttl)
		}

		c.ttl = ttl

		return nil
	}
}

// =============================================================================

// Client data structure for HTTP cache middleware.
type Client struct {
	adapter     Adapter
	cacheableFn func(*http.Request) bool
	keygenFn    func(*http.Request) (string, error)
	ttl         time.Duration
	refreshKey  string
	methods     []string
}

// NewClient initializes the cache HTTP middleware client with the given
// options.
func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	if c.adapter == nil {
		return nil, errors.New("cache client adapter is not set")
	}
	if c.cacheableFn == nil {
		c.cacheableFn = isCacheable
	}
	if c.keygenFn == nil {
		c.keygenFn = generateKey
	}
	if int64(c.ttl) < 1 {
		return nil, errors.New("cache client ttl is not set")
	}
	if c.methods == nil {
		c.methods = []string{http.MethodGet}
	}

	return c, nil
}

// Middleware is the HTTP cache middleware handler.
func (c *Client) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.cacheableFn(r) {
			ctx := r.Context()
			params := r.URL.Query()
			_, isRefresh := params[c.refreshKey]
			if isRefresh {
				delete(params, c.refreshKey)
				r.URL.RawQuery = params.Encode()
			}
			sortURLParams(r.URL)

			key, err := c.keygenFn(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if isRefresh {
				c.adapter.Release(ctx, key)
			} else {
				b, ok := c.adapter.Get(ctx, key)
				response := BytesToResponse(b)
				if ok {
					if response.Expiration.After(time.Now()) {
						response.LastAccess = time.Now()
						response.Frequency++
						c.adapter.Set(ctx, key, response.Bytes(), response.Expiration)

						//w.WriteHeader(http.StatusNotModified)
						for k, v := range response.Header {
							w.Header().Set(k, strings.Join(v, ","))
						}
						w.Write(response.Value)
						return
					}

					c.adapter.Release(ctx, key)
				}
			}

			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)
			result := rec.Result()

			statusCode := result.StatusCode
			value := rec.Body.Bytes()
			if statusCode < 400 {
				now := time.Now()

				response := Response{
					Value:      value,
					Header:     result.Header,
					Expiration: now.Add(c.ttl),
					LastAccess: now,
					Frequency:  1,
				}
				c.adapter.Set(ctx, key, response.Bytes(), response.Expiration)
			}
			for k, v := range result.Header {
				w.Header().Set(k, strings.Join(v, ","))
			}
			w.WriteHeader(statusCode)
			w.Write(value)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// =============================================================================

func generateKey(r *http.Request) (string, error) {
	if r.Method == http.MethodPost {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return "", fmt.Errorf("error reading body: %v", err)
		}
		return fmt.Sprintf("%s%s", r.URL.String(), string(body)), nil
	}
	return r.URL.String(), nil
}

func isCacheable(r *http.Request) bool {
	return r.Method == http.MethodGet
}

func sortURLParams(URL *url.URL) {
	params := URL.Query()
	for _, param := range params {
		sort.Slice(param, func(i, j int) bool {
			return param[i] < param[j]
		})
	}
	URL.RawQuery = params.Encode()
}
