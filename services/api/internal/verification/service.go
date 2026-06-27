package verification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// 验证码相关限流与有效期参数。调整时需同步邮件文案（brevo_mailer.go）与
// 前端冷却倒计时（register/page.tsx）中镜像的数值。
const (
	CodeTTL           = 10 * time.Minute // 单个验证码有效期
	CooldownWindow    = 60 * time.Second // 同一邮箱两次发码的最小间隔
	RateLimitWindow   = time.Hour        // 限流计数窗口
	MaxSendsPerHour   = 5                // 限流窗口内单个邮箱最大发码次数
	MaxVerifyAttempts = 5                // 单个验证码最大校验失败次数，超过后锁定
)

var (
	ErrInvalidEmail         = errors.New("invalid email")
	ErrCodeCooldown         = errors.New("email code cooldown")
	ErrCodeRateLimited      = errors.New("email code rate limited")
	ErrCodeInvalid          = errors.New("email code invalid")
	ErrCodeExpired          = errors.New("email code expired")
	ErrCodeAttemptsExhausted = errors.New("email code attempts exhausted")
	ErrEmailSendFailed      = errors.New("email send failed")
)

// Mailer 负责投递验证码邮件，由具体实现（如 Brevo）封装外部邮件 API。
type Mailer interface {
	SendRegistrationCode(ctx context.Context, email, code string) error
}

// Store 抽象验证码存储，便于未来从内存态切换到 SQLite 或 Redis。
type Store interface {
	Save(email string, record codeRecord)
	Latest(email, purpose string) (codeRecord, bool)
	MarkUsed(email, purpose string)
	IncrementAttempts(email, purpose string)
	RecentSendCount(email, purpose string, since time.Time) int
	Prune(now time.Time)
}

type Service struct {
	store  Store
	mailer Mailer
	now    func() time.Time
}

func NewService(store Store, mailer Mailer, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, mailer: mailer, now: now}
}

func (s *Service) SendRegistrationCode(ctx context.Context, email string) error {
	normalized := normalizeEmail(email)
	if normalized == "" || !strings.Contains(normalized, "@") {
		return ErrInvalidEmail
	}
	s.store.Prune(s.now())
	if last, ok := s.store.Latest(normalized, "register"); ok && last.SentAt.Add(CooldownWindow).After(s.now()) {
		return ErrCodeCooldown
	}
	if s.store.RecentSendCount(normalized, "register", s.now().Add(-RateLimitWindow)) >= MaxSendsPerHour {
		return ErrCodeRateLimited
	}
	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("generate verification code: %w", err)
	}
	// 先发信成功再写入 store：发信失败不应占用冷却与限流配额，否则用户重试会被
	// 冷却挡住而无法获取验证码（邮件从未投递却计入了配额）。
	if err := s.mailer.SendRegistrationCode(ctx, normalized, code); err != nil {
		return fmt.Errorf("%w: %v", ErrEmailSendFailed, err)
	}
	s.store.Save(normalized, codeRecord{
		Purpose:   "register",
		CodeHash:  hashCode(code),
		ExpiresAt: s.now().Add(CodeTTL),
		SentAt:    s.now(),
	})
	return nil
}

// generateCode 使用 crypto/rand 生成 6 位数字验证码。验证码是身份验证令牌，
// 必须使用密码学安全的随机源，避免被攻击者通过服务器时钟预测。
func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// VerifyRegistrationCode 校验验证码是否正确且未过期，但不消费验证码。
// 校验通过后调用方仍可重复校验，直到 ConsumeRegistrationCode 显式消费。
// 失败次数累计达 MaxVerifyAttempts 后锁定该验证码，即使提交正确码也拒绝，
// 需重新发码，以此抵御暴力破解。
func (s *Service) VerifyRegistrationCode(_ context.Context, email, code string) error {
	normalized := normalizeEmail(email)
	record, ok := s.store.Latest(normalized, "register")
	if !ok || !record.UsedAt.IsZero() {
		return ErrCodeInvalid
	}
	if s.now().After(record.ExpiresAt) {
		return ErrCodeExpired
	}
	if record.AttemptCount >= MaxVerifyAttempts {
		return ErrCodeAttemptsExhausted
	}
	if hashCode(strings.TrimSpace(code)) != record.CodeHash {
		s.store.IncrementAttempts(normalized, "register")
		return ErrCodeInvalid
	}
	return nil
}

// ConsumeRegistrationCode 标记该邮箱 register 用途下最新未使用的验证码为已使用。
// 应在依赖该验证码的业务操作（如创建用户）成功后调用，避免操作失败时浪费验证码。
func (s *Service) ConsumeRegistrationCode(_ context.Context, email string) {
	normalized := normalizeEmail(email)
	s.store.MarkUsed(normalized, "register")
}

// normalizeEmail 去除首尾空白并转为小写，作为验证码存储与校验的统一键。
// 归一化集中在此处，避免各调用点自行处理导致键不一致（如大小写差异）。
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func IsCooldownError(err error) bool    { return errors.Is(err, ErrCodeCooldown) }
func IsInvalidCodeError(err error) bool { return errors.Is(err, ErrCodeInvalid) }
func IsExpiredCodeError(err error) bool { return errors.Is(err, ErrCodeExpired) }

// RetryAfterSeconds 返回验证码限流类错误建议的重试秒数；非限流错误返回 0。
func RetryAfterSeconds(err error) int {
	switch {
	case errors.Is(err, ErrCodeCooldown):
		return int(CooldownWindow / time.Second)
	case errors.Is(err, ErrCodeRateLimited):
		return int(RateLimitWindow / time.Second)
	default:
		return 0
	}
}
