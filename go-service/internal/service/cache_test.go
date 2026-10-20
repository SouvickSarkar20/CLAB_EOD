package service

import (
	"testing"
	"time"

	"go-service/internal/models"
)

func TestInvalidateFilterCache(t *testing.T) {
	svc := &DataService{}

	// Populate cache manually
	globalFilterCache.mu.Lock()
	globalFilterCache.entries["test-dm"] = filterCacheEntry{
		data: &models.FilterOptions{
			Months: []string{"202501"},
		},
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	globalFilterCache.mu.Unlock()

	// Verify entry exists
	globalFilterCache.mu.RLock()
	if len(globalFilterCache.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(globalFilterCache.entries))
	}
	globalFilterCache.mu.RUnlock()

	// Invalidate cache
	svc.InvalidateFilterCache()

	// Verify cache is cleared
	globalFilterCache.mu.RLock()
	if len(globalFilterCache.entries) != 0 {
		t.Fatalf("expected 0 entries after invalidation, got %d", len(globalFilterCache.entries))
	}
	globalFilterCache.mu.RUnlock()
}

