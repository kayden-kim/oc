package stats

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

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
	client := rewriteDefaultPricingClient(t, server)

	previousNow := liteLLMNow
	liteLLMNow = func() time.Time { return time.Date(2026, time.March, 28, 1, 1, 0, 0, time.Local) }
	defer func() { liteLLMNow = previousNow }()

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, client)
	_, err := resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
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
