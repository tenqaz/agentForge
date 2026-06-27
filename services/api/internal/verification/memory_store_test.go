package verification

import (
	"testing"
	"time"
)

func TestMemoryStorePruneRemovesRecordsOutsideRateLimitWindow(t *testing.T) {
	store := NewMemoryStore(func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC) })
	base := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)

	// 处于限流窗口内的记录应保留。
	store.Save("recent@example.com", codeRecord{
		Purpose:   "register",
		SentAt:    base.Add(-30 * time.Minute),
		ExpiresAt: base.Add(-20 * time.Minute),
	})
	// 超出限流窗口的记录应被修剪。
	store.Save("old@example.com", codeRecord{
		Purpose:   "register",
		SentAt:    base.Add(-2 * time.Hour),
		ExpiresAt: base.Add(-110 * time.Minute),
	})

	store.Prune(base)

	if _, ok := store.Latest("recent@example.com", "register"); !ok {
		t.Fatalf("recent record within window should be retained")
	}
	if _, ok := store.Latest("old@example.com", "register"); ok {
		t.Fatalf("old record outside window should be pruned")
	}
	// 被全部修剪的邮箱应从 map 中删除，避免空切片残留。
	store.mu.RLock()
	_, present := store.records["old@example.com"]
	store.mu.RUnlock()
	if present {
		t.Fatalf("old email key should be deleted from map")
	}
}

func TestMemoryStorePruneRetainsUnexpiredCodeWithinWindow(t *testing.T) {
	store := NewMemoryStore(func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC) })
	base := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	// 未过期且在窗口内的记录必须保留，确保用户仍可校验。
	store.Save("active@example.com", codeRecord{
		Purpose:   "register",
		SentAt:    base.Add(-5 * time.Minute),
		ExpiresAt: base.Add(5 * time.Minute),
	})

	store.Prune(base)

	if _, ok := store.Latest("active@example.com", "register"); !ok {
		t.Fatalf("active unexpired record should be retained")
	}
}

func TestMemoryStoreIncrementAttemptsCountsFailures(t *testing.T) {
	store := NewMemoryStore(func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC) })
	store.Save("user@example.com", codeRecord{Purpose: "register"})

	for i := 0; i < 3; i++ {
		store.IncrementAttempts("user@example.com", "register")
	}

	record, ok := store.Latest("user@example.com", "register")
	if !ok {
		t.Fatalf("record missing")
	}
	if record.AttemptCount != 3 {
		t.Fatalf("AttemptCount = %d, want 3", record.AttemptCount)
	}
}
