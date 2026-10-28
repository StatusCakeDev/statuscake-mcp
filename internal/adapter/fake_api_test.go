package adapter_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/StatusCakeDev/statuscake-mcp/internal/adapter"
)

// The adapters are tested against a real SDK client talking to a fake
// StatusCake API, rather than against a mock of the SDK's interfaces. The
// SDK's request builders keep their parameters in unexported fields, which a
// mock cannot see, so a mock could not tell whether an adapter sent the right
// field. A fake server sees exactly what is sent on the wire.

// call is one request that reached the fake API.
type call struct {
	Method string
	Path   string
	Query  url.Values
	Form   url.Values
}

// fakeAPI is a StatusCake API whose routes the test declares, and which
// remembers every request it served.
type fakeAPI struct {
	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	seen   []call
}

// newAPI starts a fake API and an adapter wired to it. Retries are disabled so
// that a failing route fails once and at once.
func newAPI(t *testing.T) (*fakeAPI, *adapter.Client) {
	t.Helper()

	api := &fakeAPI{routes: map[string]http.HandlerFunc{}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()

		api.mu.Lock()
		api.seen = append(api.seen, call{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.Query(),
			Form:   r.PostForm,
		})
		mux := api.mux()
		api.mu.Unlock()

		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	client, err := adapter.New(slog.New(slog.DiscardHandler), adapter.Config{
		Token:      "token",
		RPS:        1000,
		Retries:    0,
		MinBackoff: time.Millisecond,
		MaxBackoff: time.Millisecond,
		Endpoint:   srv.URL,
	})
	if err != nil {
		t.Fatalf("adapters.New() error = %v", err)
	}

	return api, client
}

// mux builds the router from the declared routes. Anything undeclared is a 404
// with the API's error body. The caller must hold the lock.
func (a *fakeAPI) mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		write(w, http.StatusNotFound, map[string]any{"message": "not found"})
	})
	for pattern, h := range a.routes {
		mux.HandleFunc(pattern, h)
	}
	return mux
}

// serve declares a route that answers every request with the same status and
// body. The pattern is an [http.ServeMux] pattern such as "GET /v1/uptime".
// Declaring a pattern again replaces the earlier answer. A nil body sends an
// empty response.
func (a *fakeAPI) serve(pattern string, status int, body any) {
	a.serveFunc(pattern, func(*http.Request) (int, any) { return status, body })
}

// serveFunc declares a route whose answer depends on the request.
func (a *fakeAPI) serveFunc(pattern string, h func(r *http.Request) (int, any)) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.routes[pattern] = func(w http.ResponseWriter, r *http.Request) {
		status, body := h(r)
		write(w, status, body)
	}
}

// calls returns the requests served so far.
func (a *fakeAPI) calls() []call {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]call(nil), a.seen...)
}

// paths returns the distinct paths requested, sorted.
func (a *fakeAPI) paths() []string {
	var paths []string
	for _, r := range a.calls() {
		paths = append(paths, r.Path)
	}

	slices.Sort(paths)
	return slices.Compact(paths)
}

// only returns the single request that was served, failing the test if there
// were none or several.
func (a *fakeAPI) only(t *testing.T) call {
	t.Helper()

	calls := a.calls()
	if len(calls) != 1 {
		t.Fatalf("served %d requests, want exactly 1: %+v", len(calls), calls)
	}

	return calls[0]
}

func write(w http.ResponseWriter, status int, body any) {
	if body == nil {
		w.WriteHeader(status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// list wraps data in the envelope the API uses for paginated listings.
func list(pageCount int, data ...any) map[string]any {
	if data == nil {
		data = []any{}
	}

	return map[string]any{
		"data": data,
		"metadata": map[string]any{
			"page":        1,
			"per_page":    100,
			"page_count":  pageCount,
			"total_count": len(data),
		},
	}
}

// unlinked wraps data in the envelope the API uses for history and periods.
func unlinked(data ...any) map[string]any {
	if data == nil {
		data = []any{}
	}

	return map[string]any{
		"data":  data,
		"links": map[string]any{"self": "https://api.statuscake.com/v1/"},
	}
}

func item(data any) map[string]any {
	return map[string]any{"data": data}
}
