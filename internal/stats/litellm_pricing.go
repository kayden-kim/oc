package stats

import (
	_ "embed"
	"net/http"
	"sync"
	"time"
)

const liteLLMDefaultPricingURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
const liteLLMRefreshInterval = 24 * time.Hour

//go:embed model_prices_and_context_window.json
var embeddedLiteLLMPricingData []byte

var liteLLMNow = time.Now

type pricedUsage struct {
	ProviderID       string
	ModelID          string
	InputTokens      int64
	OutputTokens     int64
	ReasoningTokens  int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

type pricingResolver interface {
	EstimateCost(pricedUsage) (float64, error)
}

var defaultPricingResolver pricingResolver = newLiteLLMPricingResolver(liteLLMDefaultPricingURL, &http.Client{Timeout: 5 * time.Second})

type liteLLMPricingResolver struct {
	url    string
	client *http.Client

	initOnce    sync.Once
	mu          sync.RWMutex
	refresh     sync.Mutex
	inFlight    bool
	entries     map[string]liteLLMPricingEntry
	lastAttempt time.Time
	err         error
}

type liteLLMPricingEntry struct {
	InputCostPerToken           *float64
	OutputCostPerToken          *float64
	CacheCreationInputTokenCost *float64
	CacheReadInputTokenCost     *float64
	OutputCostPerReasoningToken *float64
	ThresholdPricing            []thresholdPricing
	TieredPricing               []tieredPricing
}

type thresholdPricing struct {
	Threshold                   int64
	InputCostPerToken           *float64
	OutputCostPerToken          *float64
	CacheCreationInputTokenCost *float64
	CacheReadInputTokenCost     *float64
}

type tieredPricing struct {
	RangeStart                  float64   `json:"range_start"`
	RangeEnd                    float64   `json:"range_end"`
	Range                       []float64 `json:"range"`
	InputCostPerToken           *float64  `json:"input_cost_per_token"`
	OutputCostPerToken          *float64  `json:"output_cost_per_token"`
	CacheCreationInputTokenCost *float64  `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     *float64  `json:"cache_read_input_token_cost"`
	OutputCostPerReasoningToken *float64  `json:"output_cost_per_reasoning_token"`
}

func estimatePartCost(event partEvent) (float64, error) {
	if defaultPricingResolver == nil {
		return 0, nil
	}

	if event.InputTokens <= 0 && event.OutputTokens <= 0 && event.ReasoningTokens <= 0 && event.CacheReadTokens <= 0 && event.CacheWriteTokens <= 0 {
		return 0, nil
	}

	return defaultPricingResolver.EstimateCost(pricedUsage{
		ProviderID:       event.ProviderID,
		ModelID:          event.ModelID,
		InputTokens:      event.InputTokens,
		OutputTokens:     event.OutputTokens,
		ReasoningTokens:  event.ReasoningTokens,
		CacheReadTokens:  event.CacheReadTokens,
		CacheWriteTokens: event.CacheWriteTokens,
	})
}

func newLiteLLMPricingResolver(url string, client *http.Client) *liteLLMPricingResolver {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &liteLLMPricingResolver{url: url, client: client}
}

func (r *liteLLMPricingResolver) EstimateCost(usage pricedUsage) (float64, error) {
	if usage.ModelID == "" {
		return 0, nil
	}

	entries, err := r.load()
	if err != nil {
		return 0, nil
	}

	entry, ok := findPricingEntry(entries, usage.ProviderID, usage.ModelID)
	if !ok {
		return 0, nil
	}

	return entry.estimateCost(usage), nil
}

func (r *liteLLMPricingResolver) load() (map[string]liteLLMPricingEntry, error) {
	r.initOnce.Do(func() {
		if r.url != liteLLMDefaultPricingURL {
			entries, _, err := r.fetchRemoteEntries()
			r.mu.Lock()
			defer r.mu.Unlock()
			r.entries = entries
			r.err = err
			return
		}

		entries, err := r.loadInitialEntries()
		r.mu.Lock()
		defer r.mu.Unlock()
		r.entries = entries
		r.err = err
	})

	r.maybeRefreshInBackground()

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.entries, r.err
}

func (r *liteLLMPricingResolver) loadInitialEntries() (map[string]liteLLMPricingEntry, error) {
	if entries, attemptedAt, err := r.loadCachedEntries(); err == nil {
		r.lastAttempt = attemptedAt
		return entries, nil
	}
	entries, err := parsePricingEntries(embeddedLiteLLMPricingData)
	if err != nil {
		return nil, err
	}
	return entries, nil
}
