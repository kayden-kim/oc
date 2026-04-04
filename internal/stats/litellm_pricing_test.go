package stats

import (
	"encoding/json"
	"errors"
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

func TestLiteLLMPricingResolver_UsesTieredRates(t *testing.T) {
	entry := liteLLMPricingEntry{
		InputCostPerToken:  ptrFloat(0.000001),
		OutputCostPerToken: ptrFloat(0.000002),
		TieredPricing: []tieredPricing{{
			Range:              []float64{0, 1000},
			InputCostPerToken:  ptrFloat(0.000003),
			OutputCostPerToken: ptrFloat(0.000004),
		}},
	}

	cost := entry.estimateCost(pricedUsage{InputTokens: 500, OutputTokens: 250})
	if cost != 0.0025 {
		t.Fatalf("expected tier-based cost 0.0025, got %.4f", cost)
	}
}

func TestFindPricingEntry_UsesProviderModelLookup(t *testing.T) {
	entries := map[string]liteLLMPricingEntry{
		"openrouter/openai/gpt-4o-mini": {InputCostPerToken: ptrFloat(0.000001)},
	}

	entry, ok := findPricingEntry(entries, "openrouter", "gpt-4o-mini")
	if !ok {
		t.Fatal("expected provider/model lookup to find entry")
	}
	if entry.InputCostPerToken == nil || *entry.InputCostPerToken != 0.000001 {
		t.Fatalf("expected provider/model lookup to return matched entry, got %#v", entry)
	}
	if _, ok := findPricingEntry(entries, "", "openai/gpt-4o-mini"); !ok {
		t.Fatal("expected suffix lookup to find provider-prefixed model")
	}
}

func TestParsePricingEntries_PreservesStringThresholdAndTierFields(t *testing.T) {
	entries, err := parsePricingEntries([]byte(`{
		"gpt-4o-mini": {
			"input_cost_per_token": "0.000001",
			"output_cost_per_token": 0.000002,
			"input_cost_per_token_above_200k_tokens": "0.000003",
			"tiered_pricing": [{
				"range": [0, 1000],
				"input_cost_per_token": 0.000004,
				"output_cost_per_token": 0.000005
			}]
		}
	}`))
	if err != nil {
		t.Fatalf("parsePricingEntries returned error: %v", err)
	}

	entry, ok := entries["gpt-4o-mini"]
	if !ok {
		t.Fatal("expected parsed entry for gpt-4o-mini")
	}
	if entry.InputCostPerToken == nil || *entry.InputCostPerToken != 0.000001 {
		t.Fatalf("expected string input cost to parse, got %#v", entry.InputCostPerToken)
	}
	if len(entry.ThresholdPricing) != 1 || entry.ThresholdPricing[0].Threshold != 200000 {
		t.Fatalf("expected parsed threshold pricing, got %#v", entry.ThresholdPricing)
	}
	if len(entry.TieredPricing) != 1 || entry.TieredPricing[0].InputCostPerToken == nil || *entry.TieredPricing[0].InputCostPerToken != 0.000004 {
		t.Fatalf("expected parsed tiered pricing, got %#v", entry.TieredPricing)
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

func TestLiteLLMPricingResolver_UsesFetchedPricingAfterBackgroundRefresh(t *testing.T) {
	writeLiteLLMCacheFixture(t, `{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`, pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 26, 9, 0, 0, 0, time.Local)})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`)
	}))
	defer server.Close()
	client := rewriteDefaultPricingClient(t, server)

	previousNow := liteLLMNow
	liteLLMNow = func() time.Time { return time.Date(2026, time.March, 27, 10, 0, 0, 0, time.Local) }
	defer func() { liteLLMNow = previousNow }()

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, client)

	firstCost, err := resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if math.Abs(firstCost-0.00001) > 1e-12 {
		t.Fatalf("expected cached price 0.00001 before refresh, got %.8f", firstCost)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		cost, err := resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
		if err != nil {
			t.Fatalf("EstimateCost returned error while waiting for refresh: %v", err)
		}
		if math.Abs(cost-0.00002) <= 1e-12 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("expected fetched pricing to replace cached pricing after background refresh")
}

func TestLiteLLMPricingResolver_UsesEmbeddedWhenCacheMissingAndRefreshFails(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	previousNow := liteLLMNow
	liteLLMNow = func() time.Time { return time.Date(2026, time.March, 27, 10, 0, 0, 0, time.Local) }
	defer func() { liteLLMNow = previousNow }()

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network down")
		}),
	})

	cost, err := resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if math.Abs(cost-0.0000015) > 1e-12 {
		t.Fatalf("expected embedded price 0.0000015, got %.10f", cost)
	}
}

func TestLiteLLMPricingResolver_ReadsContinueUsingLoadedEntriesDuringRefresh(t *testing.T) {
	writeLiteLLMCacheFixture(t, `{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`, pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 26, 9, 0, 0, 0, time.Local)})

	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		fmt.Fprint(w, `{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`)
	}))
	defer server.Close()
	client := rewriteDefaultPricingClient(t, server)

	previousNow := liteLLMNow
	liteLLMNow = func() time.Time { return time.Date(2026, time.March, 27, 10, 0, 0, 0, time.Local) }
	defer func() { liteLLMNow = previousNow }()

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, client)

	firstCost, err := resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if math.Abs(firstCost-0.00001) > 1e-12 {
		t.Fatalf("expected cached price 0.00001 before refresh, got %.8f", firstCost)
	}

	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected background refresh to start")
	}

	start := time.Now()
	secondCost, err := resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error during refresh: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected read during refresh to stay non-blocking, got %v", elapsed)
	}
	if math.Abs(secondCost-0.00001) > 1e-12 {
		t.Fatalf("expected stale cached price during refresh, got %.8f", secondCost)
	}

	close(release)
}

func TestLiteLLMPricingResolver_RefreshFailureIsBestEffortAndNonBlocking(t *testing.T) {
	writeLiteLLMCacheFixture(t, `{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`, pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 26, 9, 0, 0, 0, time.Local)})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()
	client := rewriteDefaultPricingClient(t, server)

	previousNow := liteLLMNow
	liteLLMNow = func() time.Time { return time.Date(2026, time.March, 27, 10, 0, 0, 0, time.Local) }
	defer func() { liteLLMNow = previousNow }()

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, client)

	start := time.Now()
	cost, err := resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected stale cache read to remain non-blocking, got %v", elapsed)
	}
	if math.Abs(cost-0.00001) > 1e-12 {
		t.Fatalf("expected cached price 0.00001 while refresh fails, got %.8f", cost)
	}
}

func TestLiteLLMPricingResolver_LoadCachedEntriesRequiresCompleteCacheFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	cacheDir := filepath.Join(tmp, "oc")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "litellm-pricing-cache.json"), []byte(`{"gpt-4o-mini":{"input_cost_per_token":0.000001}`), 0o644); err != nil {
		t.Fatal(err)
	}
	encodedMeta, err := json.Marshal(pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 27, 10, 0, 0, 0, time.Local)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "litellm-pricing-cache-meta.json"), encodedMeta, 0o644); err != nil {
		t.Fatal(err)
	}

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, nil)
	_, _, err = resolver.loadCachedEntries()
	if err == nil {
		t.Fatal("expected truncated cache file to fail parsing")
	}
	if _, ok := err.(*json.SyntaxError); !ok {
		t.Fatalf("expected JSON syntax error from truncated cache file, got %T", err)
	}
}

func TestCacheWrite_PreservesPreviousCacheWhenReplacementFails(t *testing.T) {
	writeLiteLLMCacheFixture(t, `{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`, pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 27, 10, 0, 0, 0, time.Local)})

	cachePath, metaPath, err := pricingCachePaths()
	if err != nil {
		t.Fatal(err)
	}
	originalCache, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	originalMeta, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}

	previousCreateTempFile := createTempFile
	previousRenameFile := renameFile
	previousReplaceFile := replaceFile
	t.Cleanup(func() {
		createTempFile = previousCreateTempFile
		renameFile = previousRenameFile
		replaceFile = previousReplaceFile
	})

	createTempFile = func(dir, pattern string) (*os.File, error) {
		return previousCreateTempFile(dir, pattern)
	}
	renameFile = func(oldPath, newPath string) error {
		return fmt.Errorf("rename failed")
	}
	replaceFile = func(oldPath, newPath string) error {
		return fmt.Errorf("replace failed")
	}

	err = writePricingCache([]byte(`{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`), time.Date(2026, time.March, 28, 10, 0, 0, 0, time.Local))
	if err == nil {
		t.Fatal("expected writePricingCache to fail when replacement rename fails")
	}

	cacheAfter, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(cacheAfter) != string(originalCache) {
		t.Fatalf("expected cache contents to stay unchanged on failed replacement, got %q", cacheAfter)
	}

	metaAfter, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(metaAfter) != string(originalMeta) {
		t.Fatalf("expected cache metadata to stay unchanged on failed replacement, got %q", metaAfter)
	}
}

func TestWritePricingCache_OverwritesExistingCacheAndMetadata(t *testing.T) {
	writeLiteLLMCacheFixture(t, `{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`, pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)})

	replacementAt := time.Date(2026, time.March, 28, 11, 0, 0, 0, time.UTC)
	if err := writePricingCache([]byte(`{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`), replacementAt); err != nil {
		t.Fatalf("writePricingCache returned error: %v", err)
	}

	cachePath, metaPath, err := pricingCachePaths()
	if err != nil {
		t.Fatal(err)
	}
	cacheData, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(cacheData) != `{"gpt-4o-mini":{"input_cost_per_token":0.000002}}` {
		t.Fatalf("expected overwritten cache contents, got %q", cacheData)
	}

	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	var meta pricingCacheMetadata
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("failed to parse metadata: %v", err)
	}
	if !meta.LastAttempt.Equal(replacementAt) {
		t.Fatalf("expected metadata timestamp %v, got %v", replacementAt, meta.LastAttempt)
	}
}

func TestWritePricingCache_HandlesOverwriteWithoutRenameSemantics(t *testing.T) {
	writeLiteLLMCacheFixture(t, `{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`, pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)})

	previousCreateTempFile := createTempFile
	previousRenameFile := renameFile
	previousReplaceFile := replaceFile
	t.Cleanup(func() {
		createTempFile = previousCreateTempFile
		renameFile = previousRenameFile
		replaceFile = previousReplaceFile
	})

	createTempFile = func(dir, pattern string) (*os.File, error) {
		return previousCreateTempFile(dir, pattern)
	}
	renameFile = func(oldPath, newPath string) error {
		return errors.New("rename cannot overwrite existing file")
	}
	replaceCalls := 0
	replaceFile = func(oldPath, newPath string) error {
		replaceCalls++
		return previousRenameFile(oldPath, newPath)
	}

	replacementAt := time.Date(2026, time.March, 28, 12, 0, 0, 0, time.UTC)
	if err := writePricingCache([]byte(`{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`), replacementAt); err != nil {
		t.Fatalf("writePricingCache returned error: %v", err)
	}
	if replaceCalls == 0 {
		t.Fatal("expected overwrite-safe replacement path to be used")
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
