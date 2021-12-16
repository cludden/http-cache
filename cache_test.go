package cache

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"
)

type adapterMock struct {
	sync.Mutex
	store map[string][]byte
}

type errReader int

func (a *adapterMock) Get(key string) ([]byte, bool) {
	a.Lock()
	defer a.Unlock()
	if _, ok := a.store[key]; ok {
		return a.store[key], true
	}
	return nil, false
}

func (a *adapterMock) Set(key string, response []byte, expiration time.Time) {
	a.Lock()
	defer a.Unlock()
	a.store[key] = response
}

func (a *adapterMock) Release(key string) {
	a.Lock()
	defer a.Unlock()
	delete(a.store, key)
}

func (errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("readAll error")
}

func TestMiddleware(t *testing.T) {
	counter := 0
	httpTestHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("new value %v", counter)))
	})

	adapter := &adapterMock{
		store: map[string][]byte{
			"http://foo.bar/test-1": Response{
				Value:      []byte("value 1"),
				Expiration: time.Now().Add(1 * time.Minute),
			}.Bytes(),
			"http://foo.bar/test-2": Response{
				Value:      []byte("value 2"),
				Expiration: time.Now().Add(1 * time.Minute),
			}.Bytes(),
			"http://foo.bar/test-3": Response{
				Value:      []byte("value 3"),
				Expiration: time.Now().Add(-1 * time.Minute),
			}.Bytes(),
			"http://foo.bar/test-4": Response{
				Value:      []byte("value 4"),
				Expiration: time.Now().Add(-1 * time.Minute),
			}.Bytes(),
		},
	}

	client, _ := NewClient(
		WithAdapter(adapter),
		WithTTL(1*time.Minute),
		WithRefreshKey("rk"),
		WithCacheable(func(r *http.Request) bool {
			return r.Method == http.MethodGet || r.Method == http.MethodPost
		}),
	)

	handler := client.Middleware(httpTestHandler)

	tests := []struct {
		name     string
		url      string
		method   string
		body     []byte
		wantBody string
		wantCode int
	}{
		{
			"returns cached response #1",
			"http://foo.bar/test-1",
			"GET",
			nil,
			"value 1",
			200,
		},
		{
			"returns new response #2",
			"http://foo.bar/test-2",
			"PUT",
			nil,
			"new value 2",
			200,
		},
		{
			"returns cached response #3",
			"http://foo.bar/test-2",
			"GET",
			nil,
			"value 2",
			200,
		},
		{
			"returns new response #4",
			"http://foo.bar/test-3?zaz=baz&baz=zaz",
			"GET",
			nil,
			"new value 4",
			200,
		},
		{
			"returns cached response #5",
			"http://foo.bar/test-3?baz=zaz&zaz=baz",
			"GET",
			nil,
			"new value 4",
			200,
		},
		{
			"cache expired #6",
			"http://foo.bar/test-3",
			"GET",
			nil,
			"new value 6",
			200,
		},
		{
			"releases cached response and returns new response #7",
			"http://foo.bar/test-2?rk=true",
			"GET",
			nil,
			"new value 7",
			200,
		},
		{
			"returns new cached response #8",
			"http://foo.bar/test-2",
			"GET",
			nil,
			"new value 7",
			200,
		},
		{
			"returns new cached response #9",
			"http://foo.bar/test-2",
			"POST",
			[]byte(`{"foo": "bar"}`),
			"new value 9",
			200,
		},
		{
			"returns new cached response #10",
			"http://foo.bar/test-2",
			"POST",
			[]byte(`{"foo": "bar"}`),
			"new value 9",
			200,
		},
		{
			"ignores request body #11",
			"http://foo.bar/test-2",
			"GET",
			[]byte(`{"foo": "bar"}`),
			"new value 7",
			200,
		},
		{
			"returns new response #12",
			"http://foo.bar/test-2",
			"POST",
			[]byte(`{"foo": "bar"}`),
			"new value 12",
			200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter++
			var r *http.Request
			var err error

			if counter != 12 {
				reader := bytes.NewReader(tt.body)
				r, err = http.NewRequest(tt.method, tt.url, reader)
				if err != nil {
					t.Error(err)
					return
				}
			} else {
				r, err = http.NewRequest(tt.method, tt.url, errReader(0))
				if err != nil {
					t.Error(err)
					return
				}
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if !reflect.DeepEqual(w.Code, tt.wantCode) {
				t.Errorf("*Client.Middleware() = %v, want %v", w.Code, tt.wantCode)
				return
			}
			if !reflect.DeepEqual(w.Body.String(), tt.wantBody) {
				t.Errorf("%s error: got '%v', expected '%v'", tt.name, w.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestBytesToResponse(t *testing.T) {
	r := Response{
		Value:      []byte("value 1"),
		Expiration: time.Time{},
		Frequency:  0,
		LastAccess: time.Time{},
	}

	tests := []struct {
		name      string
		b         []byte
		wantValue string
	}{

		{
			"convert bytes array to response",
			r.Bytes(),
			"value 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BytesToResponse(tt.b)
			if string(got.Value) != tt.wantValue {
				t.Errorf("BytesToResponse() Value = %v, want %v", got, tt.wantValue)
				return
			}
		})
	}
}

func TestResponseToBytes(t *testing.T) {
	r := Response{
		Value:      nil,
		Expiration: time.Time{},
		Frequency:  0,
		LastAccess: time.Time{},
	}

	tests := []struct {
		name     string
		response Response
	}{
		{
			"convert response to bytes array",
			r,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.response.Bytes()
			if len(b) == 0 {
				t.Error("Bytes() failed to convert")
				return
			}
		})
	}
}

func TestSortURLParams(t *testing.T) {
	u, _ := url.Parse("http://test.com?zaz=bar&foo=zaz&boo=foo&boo=baz")
	tests := []struct {
		name string
		URL  *url.URL
		want string
	}{
		{
			"returns url with ordered querystring params",
			u,
			"http://test.com?boo=baz&boo=foo&foo=zaz&zaz=bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortURLParams(tt.URL)
			got := tt.URL.String()
			if got != tt.want {
				t.Errorf("sortURLParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateKeyString(t *testing.T) {
	urls := []string{
		"http://localhost:8080/category",
		"http://localhost:8080/category/morisco",
		"http://localhost:8080/category/mourisquinho",
	}

	keys := make(map[string]string, len(urls))
	for _, u := range urls {
		r, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			t.Fatalf("error initializing request for url: %v", err)
		}

		key, _ := generateKey(r)

		if otherURL, found := keys[key]; found {
			t.Fatalf("URLs %s and %s share the same key %s", u, otherURL, key)
		}
		keys[key] = u
	}
}

func TestGenerateKey(t *testing.T) {
	tests := []struct {
		name string
		URL  string
		want string
	}{
		{
			"get url checksum",
			"http://foo.bar/test-1",
			"http://foo.bar/test-1",
		},
		{
			"get url 2 checksum",
			"http://foo.bar/test-2",
			"http://foo.bar/test-2",
		},
		{
			"get url 3 checksum",
			"http://foo.bar/test-3",
			"http://foo.bar/test-3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, tt.URL, nil)
			if err != nil {
				t.Fatalf("error initializing request for url: %v", err)
			}
			if got, _ := generateKey(r); got != tt.want {
				t.Errorf("generateKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
