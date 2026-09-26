package tools

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSerplySearchRequestAndResponse(t *testing.T) {
	t.Parallel()
	var got url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if key := r.Header.Get("X-Api-Key"); key != "test-key-serply" {
			t.Errorf("X-Api-Key = %q, want test-key-serply", key)
		}
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[
			{"title":"Go releases","link":"https://go.dev/doc/devel/release","description":" Current release notes. "},
			{"title":"","link":"https://go.dev/","description":""}
		]}`)
	}))
	defer server.Close()

	provider := &serplySearchProvider{apiKey: "test-key-serply", maxResults: 5, client: server.Client(), endpoint: server.URL}
	results, err := provider.Search(context.Background(), searchParams{
		Query: "current Go release", Count: 2, Country: "DE", SearchLang: "de", Freshness: "pw",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ key, want string }{
		{"q", "current Go release"},
		{"num", "2"},
		{"gl", "DE"},
		{"hl", "de"},
		{"tbs", "qdr:w"},
	} {
		if v := got.Get(tc.key); v != tc.want {
			t.Errorf("query %s = %q, want %q", tc.key, v, tc.want)
		}
	}

	want := []searchResult{
		{Title: "Go releases", URL: "https://go.dev/doc/devel/release", Description: "Current release notes."},
		{Title: "https://go.dev/", URL: "https://go.dev/"},
	}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("results = %#v, want %#v", results, want)
	}
}

// Serply forwards tbs to Google. The date-range freshness form has no working
// equivalent upstream, so it must not be sent as a recency filter.
func TestSerplySearchFreshnessMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		freshness string
		wantTBS   string
	}{
		{freshness: "pd", wantTBS: "qdr:d"},
		{freshness: "pw", wantTBS: "qdr:w"},
		{freshness: "pm", wantTBS: "qdr:m"},
		{freshness: "py", wantTBS: "qdr:y"},
		{freshness: "2024-01-01to2024-01-31", wantTBS: ""},
		{freshness: "nonsense", wantTBS: ""},
		{freshness: "", wantTBS: ""},
	}
	for _, tc := range tests {
		t.Run(tc.freshness, func(t *testing.T) {
			t.Parallel()
			var gotTBS string
			var tbsPresent bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotTBS = r.URL.Query().Get("tbs")
				_, tbsPresent = r.URL.Query()["tbs"]
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"results":[]}`)
			}))
			defer server.Close()

			provider := &serplySearchProvider{maxResults: 5, client: server.Client(), endpoint: server.URL}
			if _, err := provider.Search(context.Background(), searchParams{Query: "q", Count: 1, Freshness: tc.freshness}); err != nil {
				t.Fatal(err)
			}
			if gotTBS != tc.wantTBS {
				t.Errorf("tbs = %q, want %q", gotTBS, tc.wantTBS)
			}
			if tc.wantTBS == "" && tbsPresent {
				t.Errorf("tbs sent as empty param, want omitted entirely")
			}
		})
	}
}

func TestSerplySearchAppliesLimitAndSkipsMissingLink(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[
			{"title":"missing link","link":"","description":"skip"},
			{"title":"one","link":"https://example.com/1","description":"one"},
			{"title":"two","link":"https://example.com/2","description":"two"}
		]}`)
	}))
	defer server.Close()

	provider := &serplySearchProvider{maxResults: 1, client: server.Client(), endpoint: server.URL}
	results, err := provider.Search(context.Background(), searchParams{Query: "q", Count: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "one" {
		t.Fatalf("results = %#v, want first valid result", results)
	}
}

func TestSerplySearchResponseErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "invalid key", status: http.StatusUnauthorized, body: `{"detail":"Invalid API key"}`, want: "returned 401"},
		{name: "server error", status: http.StatusServiceUnavailable, body: ``, want: "returned 503"},
		{name: "malformed body", status: http.StatusOK, body: `{`, want: "parse response"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()

			provider := &serplySearchProvider{maxResults: 1, client: server.Client(), endpoint: server.URL}
			_, err := provider.Search(context.Background(), searchParams{Query: "q", Count: 1})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestSerplySearchPropagatesCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	client := &http.Client{Transport: serplyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	provider := &serplySearchProvider{maxResults: 1, client: client, endpoint: "https://api.serply.io/v1/search"}
	_, err := provider.Search(ctx, searchParams{Query: "q", Count: 1})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

// Serply is key-gated, so unlike Parallel it belongs in the default order: a
// tenant with no key is skipped by BuildChainFromStorage rather than by being
// absent from the order.
func TestSerplyProviderIsInDefaultOrder(t *testing.T) {
	if got := NormalizeWebSearchProviderOrder(nil); !containsString(got, searchProviderSerply) {
		t.Fatalf("default provider order missing serply: %v", got)
	}
	got := NormalizeWebSearchProviderOrder([]string{searchProviderSerply})
	if len(got) == 0 || got[0] != searchProviderSerply {
		t.Fatalf("explicit provider order = %v, want serply first", got)
	}
	if provider := buildProviderByName(searchProviderSerply, "k", 3); provider == nil || provider.Name() != searchProviderSerply {
		t.Fatalf("serply provider construction failed: %T", provider)
	}
}

type serplyRoundTripFunc func(*http.Request) (*http.Response, error)

func (f serplyRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
