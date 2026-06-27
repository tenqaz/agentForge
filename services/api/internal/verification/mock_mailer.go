package verification

import (
	"context"
	"log/slog"
)

// MockMailer 是一个用于开发/测试的邮件发送器，不实际发送邮件。
// 验证码固定为 123456，方便在未配置 Brevo 时进行本地开发和测试。
type MockMailer struct{}

func NewMockMailer() *MockMailer {
	return &MockMailer{}
}

func (m *MockMailer) SendRegistrationCode(ctx context.Context, email, code string) error {
	slog.Info("Mock email sent (not actually delivered)",
		"to", email,
		"code", "123456",
		"note", "actual code ignored, always use 123456")
	return nil
}
