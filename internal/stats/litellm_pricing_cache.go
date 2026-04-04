package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type pricingCacheMetadata struct {
	LastAttempt time.Time `json:"last_attempt"`
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
