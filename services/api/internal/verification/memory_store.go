package verification

import (
	"context"
	"sync"
	"time"
)

// codeRecord 保存单个验证码的状态与发送历史。
type codeRecord struct {
	Purpose      string
	CodeHash     string
	ExpiresAt    time.Time
	SentAt       time.Time
	UsedAt       time.Time
	AttemptCount int
}

// MemoryStore 是单进程内存实现的验证码存储，使用读写锁保证并发安全。
type MemoryStore struct {
	mu      sync.RWMutex
	now     func() time.Time
	records map[string][]codeRecord
}

func NewMemoryStore(now func() time.Time) *MemoryStore {
	if now == nil {
		now = time.Now
	}
	return &MemoryStore{now: now, records: map[string][]codeRecord{}}
}

func (s *MemoryStore) Save(email string, record codeRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[email] = append(s.records[email], record)
}

func (s *MemoryStore) Latest(email, purpose string) (codeRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := s.records[email]
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Purpose == purpose {
			return records[i], true
		}
	}
	return codeRecord{}, false
}

func (s *MemoryStore) MarkUsed(email, purpose string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := s.records[email]
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Purpose == purpose && records[i].UsedAt.IsZero() {
			records[i].UsedAt = s.now()
			return
		}
	}
}

// IncrementAttempts 递增该邮箱指定用途下最新未使用验证码的失败尝试次数，
// 用于 VerifyRegistrationCode 的暴力破解防护。
func (s *MemoryStore) IncrementAttempts(email, purpose string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := s.records[email]
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Purpose == purpose && records[i].UsedAt.IsZero() {
			records[i].AttemptCount++
			return
		}
	}
}

func (s *MemoryStore) RecentSendCount(email, purpose string, since time.Time) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, record := range s.records[email] {
		if record.Purpose == purpose && !record.SentAt.Before(since) {
			count++
		}
	}
	return count
}

// Prune 移除发送时间早于一个限流窗口的记录。RecentSendCount 只统计窗口内的
// 发送次数，超出窗口的记录对限流无意义，丢弃以释放内存。由于 CodeTTL 短于
// RateLimitWindow，未过期的验证码必然仍在窗口内，不会被误删。
func (s *MemoryStore) Prune(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for email, records := range s.records {
		kept := records[:0]
		// 原地过滤：kept 的写入游标始终不超前 range 的读取游标，复用底层数组安全。
		for _, record := range records {
			if now.Sub(record.SentAt) <= RateLimitWindow {
				kept = append(kept, record)
			}
		}
		if len(kept) == 0 {
			delete(s.records, email)
			continue
		}
		s.records[email] = kept
	}
}

// RunCleanup 周期性清理过期记录，应在后台 goroutine 中运行，ctx 取消时退出。
// 避免无人发码时 Prune 不被触发、过期记录持续占用内存。
func (s *MemoryStore) RunCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Prune(s.now())
		}
	}
}
