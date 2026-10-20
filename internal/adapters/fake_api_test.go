package adapters_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"

	"github.com/StatusCakeDev/statuscake-go"

	"github.com/StatusCakeDev/statuscake-mcp/internal/adapters"
)

// The adapter is tested against a real SDK client talking to a fake StatusCake
// API, rather than against a mock of the SDK's interfaces. The SDK's request
// builders keep their filters in unexported fields, which a mock cannot see, so
// a mock could not tell whether the adapter asked for the right page or the
// right window. A fake server sees exactly what is sent on the wire.

// recorded is one request that reached the fake API.
type recorded struct {
	Path  string
	Query url.Values
}

// fakeAPI is a StatusCake API whose routes the test declares, and which
// remembers every request it served.
type fakeAPI struct {
	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	seen   []recorded
}

// newAPI starts a fake API and a client wired to it. Retries are disabled so
// that a failing route fails once and at once.
func newAPI(t *testing.T) (*fakeAPI, *adapters.Client) {
	t.Helper()

	api := &fakeAPI{routes: map[string]http.HandlerFunc{}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.mu.Lock()
		api.seen = append(api.seen, recorded{Path: r.URL.Path, Query: r.URL.Query()})
		mux := api.mux()
		api.mu.Unlock()

		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	sdk := statuscake.NewClient(
		statuscake.WithHost(srv.URL),
		statuscake.WithDisableRetry(),
	)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return api, adapters.NewClient(logger, sdk)
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
// Declaring a pattern again replaces the earlier answer.
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

// requests returns every request served so far, in order.
func (a *fakeAPI) requests() []recorded {
	a.mu.Lock()
	defer a.mu.Unlock()

	return slices.Clone(a.seen)
}

// paths returns the distinct paths requested, sorted.
func (a *fakeAPI) paths() []string {
	var paths []string
	for _, r := range a.requests() {
		paths = append(paths, r.Path)
	}

	slices.Sort(paths)
	return slices.Compact(paths)
}

// only returns the single request served, failing the test if there was not
// exactly one.
func (a *fakeAPI) only(t *testing.T) recorded {
	t.Helper()

	reqs := a.requests()
	if len(reqs) != 1 {
		t.Fatalf("served %d requests, want exactly 1: %v", len(reqs), reqs)
	}
	return reqs[0]
}

func write(w http.ResponseWriter, status int, body any) {
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
