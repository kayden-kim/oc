package stats

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

type pricingCacheMetadata struct {
	LastAttempt time.Time `json:"last_attempt"`
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

func (r *liteLLMPricingResolver) maybeRefreshInBackground() {
	if r.url != liteLLMDefaultPricingURL {
		return
	}
	now := liteLLMNow()
	r.mu.RLock()
	lastAttempt := r.lastAttempt
	r.mu.RUnlock()
	if !lastAttempt.IsZero() && now.Sub(lastAttempt) < liteLLMRefreshInterval {
		return
	}

	r.refresh.Lock()
	if r.inFlight {
		r.refresh.Unlock()
		return
	}
	r.inFlight = true
	r.refresh.Unlock()

	r.mu.Lock()
	r.lastAttempt = now
	r.mu.Unlock()
	_ = writePricingCacheMetadata(now)

	go func() {
		defer func() {
			r.refresh.Lock()
			r.inFlight = false
			r.refresh.Unlock()
		}()

		entries, raw, err := r.fetchRemoteEntries()
		if err != nil {
			return
		}
		_ = writePricingCache(raw, now)
		r.mu.Lock()
		r.entries = entries
		r.err = nil
		r.lastAttempt = now
		r.mu.Unlock()
	}()
}

func (r *liteLLMPricingResolver) fetchRemoteEntries() (map[string]liteLLMPricingEntry, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected pricing status: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	entries, err := parsePricingEntries(body)
	if err != nil {
		return nil, nil, err
	}
	return entries, body, nil
}

func parsePricingEntries(data []byte) (map[string]liteLLMPricingEntry, error) {
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	entries := make(map[string]liteLLMPricingEntry, len(raw))
	for modelName, payload := range raw {
		entries[modelName] = parseLiteLLMPricingEntry(payload)
	}
	return entries, nil
}

func (r *liteLLMPricingResolver) loadCachedEntries() (map[string]liteLLMPricingEntry, time.Time, error) {
	cachePath, metaPath, err := pricingCachePaths()
	if err != nil {
		return nil, time.Time{}, err
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, time.Time{}, err
	}
	entries, err := parsePricingEntries(data)
	if err != nil {
		return nil, time.Time{}, err
	}
	meta, err := os.ReadFile(metaPath)
	if err != nil {
		return entries, time.Time{}, nil
	}
	var cached pricingCacheMetadata
	if err := json.Unmarshal(meta, &cached); err != nil {
		return entries, time.Time{}, nil
	}
	return entries, cached.LastAttempt, nil
}

func writePricingCache(raw []byte, attemptedAt time.Time) error {
	cachePath, _, err := pricingCachePaths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cachePath, raw, 0o644); err != nil {
		return err
	}
	return writePricingCacheMetadata(attemptedAt)
}

func writePricingCacheMetadata(attemptedAt time.Time) error {
	_, metaPath, err := pricingCachePaths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(pricingCacheMetadata{LastAttempt: attemptedAt})
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, payload, 0o644)
}

func pricingCachePaths() (string, string, error) {
	root, err := ocDataDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(root, "litellm-pricing-cache.json"), filepath.Join(root, "litellm-pricing-cache-meta.json"), nil
}

func ocDataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "oc"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "oc"), nil
}

func parseLiteLLMPricingEntry(payload map[string]json.RawMessage) liteLLMPricingEntry {
	entry := liteLLMPricingEntry{
		InputCostPerToken:           rawFloat(payload, "input_cost_per_token"),
		OutputCostPerToken:          rawFloat(payload, "output_cost_per_token"),
		CacheCreationInputTokenCost: rawFloat(payload, "cache_creation_input_token_cost"),
		CacheReadInputTokenCost:     rawFloat(payload, "cache_read_input_token_cost"),
		OutputCostPerReasoningToken: rawFloat(payload, "output_cost_per_reasoning_token"),
		TieredPricing:               rawTiers(payload, "tiered_pricing"),
	}

	entry.ThresholdPricing = rawThresholds(payload)
	return entry
}

func rawFloat(payload map[string]json.RawMessage, key string) *float64 {
	raw, ok := payload[key]
	if !ok {
		return nil
	}

	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		return &number
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		parsed, err := strconv.ParseFloat(asString, 64)
		if err == nil {
			return &parsed
		}
	}

	return nil
}

func rawTiers(payload map[string]json.RawMessage, key string) []tieredPricing {
	raw, ok := payload[key]
	if !ok {
		return nil
	}

	var tiers []tieredPricing
	if err := json.Unmarshal(raw, &tiers); err != nil {
		return nil
	}
	return tiers
}

func rawThresholds(payload map[string]json.RawMessage) []thresholdPricing {
	thresholds := map[int64]*thresholdPricing{}

	for key := range payload {
		const suffix = "_tokens"
		idx := strings.Index(key, "_above_")
		if idx == -1 || !strings.HasSuffix(key, suffix) {
			continue
		}

		thresholdPart := strings.TrimSuffix(key[idx+len("_above_"):], suffix)
		threshold, err := parseTokenThreshold(thresholdPart)
		if err != nil {
			continue
		}

		bucket := thresholds[threshold]
		if bucket == nil {
			bucket = &thresholdPricing{Threshold: threshold}
			thresholds[threshold] = bucket
		}

		switch strings.TrimSuffix(key[:idx], "_") {
		case "input_cost_per_token":
			bucket.InputCostPerToken = rawFloat(payload, key)
		case "output_cost_per_token":
			bucket.OutputCostPerToken = rawFloat(payload, key)
		case "cache_creation_input_token_cost":
			bucket.CacheCreationInputTokenCost = rawFloat(payload, key)
		case "cache_read_input_token_cost":
			bucket.CacheReadInputTokenCost = rawFloat(payload, key)
		}
	}

	result := make([]thresholdPricing, 0, len(thresholds))
	for _, threshold := range thresholds {
		result = append(result, *threshold)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Threshold < result[j].Threshold
	})
	return result
}

func parseTokenThreshold(raw string) (int64, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	multiplier := int64(1)
	if strings.HasSuffix(raw, "k") {
		multiplier = 1000
		raw = strings.TrimSuffix(raw, "k")
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return n * multiplier, nil
}

func findPricingEntry(entries map[string]liteLLMPricingEntry, providerID, modelID string) (liteLLMPricingEntry, bool) {
	for _, candidate := range pricingCandidates(providerID, modelID) {
		if entry, ok := entries[candidate]; ok {
			return entry, true
		}
	}

	modelLower := strings.ToLower(trimProviderPrefix(modelID))
	for key, entry := range entries {
		keyLower := strings.ToLower(key)
		if keyLower == modelLower || strings.HasSuffix(keyLower, "/"+modelLower) {
			return entry, true
		}
	}

	return liteLLMPricingEntry{}, false
}

func pricingCandidates(providerID, modelID string) []string {
	modelID = strings.TrimSpace(modelID)
	providerID = strings.TrimSpace(strings.ToLower(providerID))
	modelPart := trimProviderPrefix(modelID)

	base := []string{modelID, modelPart}
	base = append(base, aliasModelCandidates(modelPart)...)

	prefixes := []string{"anthropic/", "openai/", "azure/", "openrouter/openai/", "vertex_ai/", "bedrock/", "gemini/", "google/"}
	if providerID != "" {
		prefixes = append([]string{providerID + "/"}, prefixes...)
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(base)*(len(prefixes)+1))
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	for _, candidate := range base {
		add(candidate)
		for _, prefix := range prefixes {
			add(prefix + candidate)
		}
	}

	return result
}

func trimProviderPrefix(modelID string) string {
	if idx := strings.LastIndex(modelID, "/"); idx >= 0 {
		return modelID[idx+1:]
	}
	return modelID
}

func aliasModelCandidates(modelID string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	add := func(value string) {
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	add(modelID)
	if strings.HasSuffix(modelID, "-high") {
		add(strings.TrimSuffix(modelID, "-high"))
	}
	if strings.HasSuffix(modelID, "-thinking") {
		add(strings.TrimSuffix(modelID, "-thinking"))
	}
	if modelID == "gemini-3-pro-high" {
		add("gemini-3-pro-preview")
	}
	return result
}

func (entry liteLLMPricingEntry) estimateCost(usage pricedUsage) float64 {
	if tier, ok := entry.matchTier(usage.InputTokens); ok {
		return calculateCostForRates(usage, tier.InputCostPerToken, tier.OutputCostPerToken, tier.CacheCreationInputTokenCost, tier.CacheReadInputTokenCost, tier.OutputCostPerReasoningToken)
	}

	inputRate := entry.rateForThreshold(usage.InputTokens, entry.InputCostPerToken, func(tp thresholdPricing) *float64 { return tp.InputCostPerToken })
	outputRate := entry.rateForThreshold(usage.OutputTokens, entry.OutputCostPerToken, func(tp thresholdPricing) *float64 { return tp.OutputCostPerToken })
	cacheWriteRate := entry.rateForThreshold(usage.InputTokens, entry.CacheCreationInputTokenCost, func(tp thresholdPricing) *float64 { return tp.CacheCreationInputTokenCost })
	cacheReadRate := entry.rateForThreshold(usage.InputTokens, entry.CacheReadInputTokenCost, func(tp thresholdPricing) *float64 { return tp.CacheReadInputTokenCost })

	return calculateCostForRates(usage, inputRate, outputRate, cacheWriteRate, cacheReadRate, entry.OutputCostPerReasoningToken)
}

func (entry liteLLMPricingEntry) matchTier(promptTokens int64) (tieredPricing, bool) {
	if len(entry.TieredPricing) == 0 {
		return tieredPricing{}, false
	}

	prompt := float64(promptTokens)
	for _, tier := range entry.TieredPricing {
		start, end := tierRange(tier)
		if prompt >= start && prompt < end {
			return tier, true
		}
	}
	return tieredPricing{}, false
}

func tierRange(tier tieredPricing) (float64, float64) {
	if len(tier.Range) == 2 {
		return tier.Range[0], tier.Range[1]
	}
	return tier.RangeStart, tier.RangeEnd
}

func (entry liteLLMPricingEntry) rateForThreshold(tokens int64, base *float64, pick func(thresholdPricing) *float64) *float64 {
	selected := base
	for _, threshold := range entry.ThresholdPricing {
		if tokens <= threshold.Threshold {
			continue
		}
		if candidate := pick(threshold); candidate != nil {
			selected = candidate
		}
	}
	return selected
}

func calculateCostForRates(usage pricedUsage, inputRate, outputRate, cacheWriteRate, cacheReadRate, reasoningRate *float64) float64 {
	total := 0.0
	total += costForTokens(usage.InputTokens, inputRate)
	total += costForTokens(usage.OutputTokens, outputRate)
	total += costForTokens(usage.CacheWriteTokens, cacheWriteRate)
	total += costForTokens(usage.CacheReadTokens, cacheReadRate)
	total += costForTokens(usage.ReasoningTokens, reasoningRate)
	return total
}

func costForTokens(tokens int64, rate *float64) float64 {
	if tokens <= 0 || rate == nil {
		return 0
	}
	return float64(tokens) * *rate
}
