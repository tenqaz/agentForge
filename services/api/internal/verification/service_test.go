package verification

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubMailer struct {
	lastEmail string
	lastCode  string
	err       error
}

func (m *stubMailer) SendRegistrationCode(_ context.Context, email, code string) error {
	m.lastEmail = email
	m.lastCode = code
	return m.err
}

func TestServiceSendRegistrationCodeEnforcesCooldown(t *testing.T) {
	store := NewMemoryStore(func() time.Time {
		return time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	})
	mailer := &stubMailer{}
	service := NewService(store, mailer, func() time.Time {
		return time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	})

	if err := service.SendRegistrationCode(context.Background(), " USER@example.com "); err != nil {
		t.Fatalf("first send error = %v", err)
	}
	if mailer.lastEmail != "user@example.com" {
		t.Fatalf("lastEmail = %q", mailer.lastEmail)
	}
	if len(mailer.lastCode) != 6 {
		t.Fatalf("lastCode length = %d", len(mailer.lastCode))
	}

	err := service.SendRegistrationCode(context.Background(), "user@example.com")
	if err == nil || !IsCooldownError(err) {
		t.Fatalf("second send error = %v, want cooldown", err)
	}
}

func TestServiceVerifyRegistrationCodeRejectsExpiredAndStaysValidUntilConsumed(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return now })
	mailer := &stubMailer{}
	service := NewService(store, mailer, func() time.Time { return now })

	if err := service.SendRegistrationCode(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("send error = %v", err)
	}
	code := mailer.lastCode

	if err := service.VerifyRegistrationCode(context.Background(), "user@example.com", "000000"); err == nil || !IsInvalidCodeError(err) {
		t.Fatalf("invalid verify error = %v, want invalid", err)
	}

	// 校验通过不会消费验证码，同一码可重复校验直到被显式消费。
	if err := service.VerifyRegistrationCode(context.Background(), "user@example.com", code); err != nil {
		t.Fatalf("verify error = %v", err)
	}
	if err := service.VerifyRegistrationCode(context.Background(), "user@example.com", code); err != nil {
		t.Fatalf("reuse verify before consume error = %v, want still valid", err)
	}

	// 显式消费后，验证码失效，再次校验返回 invalid。
	service.ConsumeRegistrationCode(context.Background(), "user@example.com")
	if err := service.VerifyRegistrationCode(context.Background(), "user@example.com", code); err == nil || !IsInvalidCodeError(err) {
		t.Fatalf("reuse after consume error = %v, want invalid", err)
	}

	// 过期验证码返回 expired。
	expiredNow := now.Add(11 * time.Minute)
	expiredService := NewService(store, mailer, func() time.Time { return expiredNow })
	if err := expiredService.SendRegistrationCode(context.Background(), "other@example.com"); err != nil {
		t.Fatalf("send second email error = %v", err)
	}
	expiredCode := mailer.lastCode
	expiredService = NewService(store, mailer, func() time.Time { return expiredNow.Add(11 * time.Minute) })
	if err := expiredService.VerifyRegistrationCode(context.Background(), "other@example.com", expiredCode); err == nil || !IsExpiredCodeError(err) {
		t.Fatalf("expired verify error = %v, want expired", err)
	}
}

func TestServiceSendRegistrationCodeDoesNotReserveCooldownWhenSendFails(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return now })
	// 第一次发信失败（模拟 Brevo 不可达）。
	failingMailer := &stubMailer{err: errors.New("brevo unreachable")}
	service := NewService(store, failingMailer, func() time.Time { return now })

	if err := service.SendRegistrationCode(context.Background(), "user@example.com"); err == nil || !errors.Is(err, ErrEmailSendFailed) {
		t.Fatalf("first send error = %v, want ErrEmailSendFailed", err)
	}

	// 发信失败不应占用冷却：立即用成功 mailer 重发应被接受。
	successMailer := &stubMailer{}
	service = NewService(store, successMailer, func() time.Time { return now })
	if err := service.SendRegistrationCode(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("retry send error = %v, want nil (cooldown should not be reserved on send failure)", err)
	}
	if successMailer.lastCode == "" {
		t.Fatalf("retry should have sent a code")
	}
}

func TestServiceSendRegistrationCodeEnforcesHourlyRateLimit(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return now })
	mailer := &stubMailer{}
	service := NewService(store, mailer, func() time.Time { return now })

	// 每次推进 61 秒绕过冷却，但仍处于 1 小时限流窗口内。
	for i := 0; i < MaxSendsPerHour; i++ {
		if err := service.SendRegistrationCode(context.Background(), "user@example.com"); err != nil {
			t.Fatalf("send %d error = %v", i+1, err)
		}
		now = now.Add(61 * time.Second)
	}

	// 第 MaxSendsPerHour+1 次被限流（与冷却无关，独立的安全控制）。
	err := service.SendRegistrationCode(context.Background(), "user@example.com")
	if err == nil || !errors.Is(err, ErrCodeRateLimited) {
		t.Fatalf("rate limited error = %v, want ErrCodeRateLimited", err)
	}
}

func TestServiceVerifyRegistrationCodeLocksAfterMaxAttempts(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return now })
	mailer := &stubMailer{}
	service := NewService(store, mailer, func() time.Time { return now })

	if err := service.SendRegistrationCode(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("send error = %v", err)
	}
	correct := mailer.lastCode

	// 前 MaxVerifyAttempts 次错误校验返回 invalid。
	for i := 0; i < MaxVerifyAttempts; i++ {
		err := service.VerifyRegistrationCode(context.Background(), "user@example.com", "000000")
		if err == nil || !IsInvalidCodeError(err) {
			t.Fatalf("attempt %d error = %v, want invalid", i+1, err)
		}
	}

	// 超过上限后锁定：即使提交正确码也返回 exhausted，需重新发码。
	err := service.VerifyRegistrationCode(context.Background(), "user@example.com", correct)
	if err == nil || !errors.Is(err, ErrCodeAttemptsExhausted) {
		t.Fatalf("locked verify error = %v, want ErrCodeAttemptsExhausted", err)
	}
}
