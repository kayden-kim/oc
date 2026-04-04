package stats

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

	previousCreateTempFile := atomicWriteCreateTempFile
	previousRenameFile := atomicWriteRenameFile
	previousReplaceFile := atomicReplaceFile
	t.Cleanup(func() {
		atomicWriteCreateTempFile = previousCreateTempFile
		atomicWriteRenameFile = previousRenameFile
		atomicReplaceFile = previousReplaceFile
	})

	atomicWriteCreateTempFile = func(dir, pattern string) (*os.File, error) {
		return previousCreateTempFile(dir, pattern)
	}
	atomicWriteRenameFile = func(oldPath, newPath string) error {
		return fmt.Errorf("rename failed")
	}
	atomicReplaceFile = func(oldPath, newPath string) error {
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

	previousCreateTempFile := atomicWriteCreateTempFile
	previousRenameFile := atomicWriteRenameFile
	previousReplaceFile := atomicReplaceFile
	t.Cleanup(func() {
		atomicWriteCreateTempFile = previousCreateTempFile
		atomicWriteRenameFile = previousRenameFile
		atomicReplaceFile = previousReplaceFile
	})

	atomicWriteCreateTempFile = func(dir, pattern string) (*os.File, error) {
		return previousCreateTempFile(dir, pattern)
	}
	atomicWriteRenameFile = func(oldPath, newPath string) error {
		return errors.New("rename cannot overwrite existing file")
	}
	replaceCalls := 0
	atomicReplaceFile = func(oldPath, newPath string) error {
		replaceCalls++
		return previousRenameFile(oldPath, newPath)
	}

	if err := writePricingCache([]byte(`{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`), time.Date(2026, time.March, 28, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("writePricingCache returned error: %v", err)
	}
	if replaceCalls == 0 {
		t.Fatal("expected overwrite-safe replacement path to be used")
	}
}
