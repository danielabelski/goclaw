package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

func TestOpenAIEmbeddingProviderRestoresResponseOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 1, "embedding": []float32{2, 2, 2}},
				{"index": 0, "embedding": []float32{1, 1, 1}},
			},
		})
	}))
	defer server.Close()

	provider := NewOpenAIEmbeddingProvider("test", "key", server.URL, "model").WithDimensions(3)
	embeddings, err := provider.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if embeddings[0][0] != 1 || embeddings[1][0] != 2 {
		t.Fatalf("embeddings returned in response order: %v", embeddings)
	}
}

func TestOpenAIEmbeddingProviderRejectsWrongDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,2]}]}`))
	}))
	defer server.Close()

	provider := NewOpenAIEmbeddingProvider("test", "key", server.URL, "model").WithDimensions(3)
	if _, err := provider.Embed(context.Background(), []string{"text"}); err == nil {
		t.Fatal("Embed() error = nil, want dimension validation error")
	}
}

func TestOpenAIEmbeddingProviderUsesBoundedHTTPClient(t *testing.T) {
	provider := NewOpenAIEmbeddingProvider("test", "key", "http://example.invalid", "model")
	if provider.httpClient == nil || provider.httpClient.Timeout != 60*time.Second {
		t.Fatalf("HTTP timeout = %v, want 60s", provider.httpClient)
	}
}

func TestOpenAIEmbeddingProviderRewritesLoopbackInDocker(t *testing.T) {
	restore := config.SetInDockerForTest(true)
	defer restore()

	provider := NewOpenAIEmbeddingProvider("ollama", "", "http://localhost:11434/v1", "nomic-embed-text")
	if provider.apiURL != "http://host.docker.internal:11434/v1" {
		t.Fatalf("apiURL = %q, want Docker localhost rewrite applied in constructor", provider.apiURL)
	}
}

func TestOpenAIEmbeddingProviderKeepsLoopbackOutsideDocker(t *testing.T) {
	restore := config.SetInDockerForTest(false)
	defer restore()

	provider := NewOpenAIEmbeddingProvider("ollama", "", "http://localhost:11434/v1", "nomic-embed-text")
	if provider.apiURL != "http://localhost:11434/v1" {
		t.Fatalf("apiURL = %q, want unchanged outside Docker", provider.apiURL)
	}
}

// TestMain pins Docker detection off so the httptest-based Embed tests above
// stay deterministic even when the test binary itself runs inside a container.
func TestMain(m *testing.M) {
	restore := config.SetInDockerForTest(false)
	code := m.Run()
	restore()
	os.Exit(code)
}
