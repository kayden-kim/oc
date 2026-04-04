package stats

import (
	"fmt"
	"io"
	"net/http"
)

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
