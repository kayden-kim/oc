package stats

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestLiteLLMPricingResolver_UsesThresholdRates(t *testing.T) {
	entry := liteLLMPricingEntry{
		InputCostPerToken: ptrFloat(0.000001),
		ThresholdPricing: []thresholdPricing{{
			Threshold:         200000,
			InputCostPerToken: ptrFloat(0.000002),
		}},
	}

	cost := entry.estimateCost(pricedUsage{InputTokens: 300000})
	if cost != 0.6 {
		t.Fatalf("expected threshold-based cost 0.6, got %.4f", cost)
	}
}

func TestLiteLLMPricingResolver_UsesAliasMapping(t *testing.T) {
	resolver := &liteLLMPricingResolver{
		entries: map[string]liteLLMPricingEntry{
			"gemini-3-pro-preview": {InputCostPerToken: ptrFloat(0.000001)},
		},
	}
	resolver.initOnce.Do(func() {})

	cost, err := resolver.EstimateCost(pricedUsage{ModelID: "gemini-3-pro-high", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if math.Abs(cost-0.00001) > 1e-12 {
		t.Fatalf("expected alias-based cost 0.00001, got %.8f", cost)
	}
}

func TestLiteLLMPricingResolver_UsesLocalCacheImmediately(t *testing.T) {
	writeLiteLLMCacheFixture(t, `{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`, pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 26, 9, 0, 0, 0, time.Local)})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, `{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`)
	}))
	defer server.Close()

	previousNow := liteLLMNow
	liteLLMNow = func() time.Time { return time.Date(2026, time.March, 27, 10, 0, 0, 0, time.Local) }
	defer func() { liteLLMNow = previousNow }()

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, server.Client())
	start := time.Now()
	cost, err := resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected cached pricing to return immediately, got %v", elapsed)
	}
	if math.Abs(cost-0.00001) > 1e-12 {
		t.Fatalf("expected cached price 0.00001, got %.8f", cost)
	}
}

func TestLiteLLMPricingResolver_RefreshesAtMostOncePerRollingWindow(t *testing.T) {
	writeLiteLLMCacheFixture(t, `{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`, pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 27, 1, 0, 0, 0, time.Local)})

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		fmt.Fprint(w, `{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`)
	}))
	defer server.Close()
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

	previousNow := liteLLMNow
	liteLLMNow = func() time.Time { return time.Date(2026, time.March, 28, 1, 1, 0, 0, time.Local) }
	defer func() { liteLLMNow = previousNow }()

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, client)
	_, err = resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && hits.Load() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected one refresh after rolling window expired, got %d fetches", hits.Load())
	}
}

func TestEstimatePartCost_UsesPricingFallback(t *testing.T) {
	previousResolver := defaultPricingResolver
	t.Cleanup(func() {
		defaultPricingResolver = previousResolver
	})

	resolver := &liteLLMPricingResolver{
		entries: map[string]liteLLMPricingEntry{
			"gpt-4o-mini": {
				InputCostPerToken:           ptrFloat(0.000001),
				OutputCostPerToken:          ptrFloat(0.000002),
				CacheCreationInputTokenCost: ptrFloat(0.000003),
				CacheReadInputTokenCost:     ptrFloat(0.0000005),
			},
		},
	}
	resolver.initOnce.Do(func() {})
	defaultPricingResolver = resolver

	cost, err := estimatePartCost(partEvent{
		ProviderID:       "openai",
		ModelID:          "gpt-4o-mini",
		InputTokens:      1000,
		OutputTokens:     500,
		CacheReadTokens:  200,
		CacheWriteTokens: 100,
	})
	if err != nil {
		t.Fatalf("estimatePartCost returned error: %v", err)
	}

	const expected = 0.0024
	if math.Abs(cost-expected) > 1e-9 {
		t.Fatalf("expected estimated fallback cost %.4f, got %.4f", expected, cost)
	}
}

func ptrFloat(value float64) *float64 {
	return &value
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
