package stats

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func ptrFloat(value float64) *float64 {
	return &value
}

func rewriteDefaultPricingClient(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	transport := client.Transport
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = serverURL.Scheme
		clone.URL.Host = serverURL.Host
		return transport.RoundTrip(clone)
	})
	return client
}

func writeLiteLLMCacheFixture(t *testing.T, cacheJSON string, meta pricingCacheMetadata) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	cacheDir := filepath.Join(tmp, "oc")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "litellm-pricing-cache.json"), []byte(cacheJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	encodedMeta, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "litellm-pricing-cache-meta.json"), encodedMeta, 0o644); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
