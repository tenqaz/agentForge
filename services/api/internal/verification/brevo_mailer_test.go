package verification

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrevoMailerIncludesResponseBodyOnError(t *testing.T) {
	// 模拟 Brevo 返回 401 + JSON 错误体（含具体原因）。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") == "" {
			t.Errorf("request missing api-key header")
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"code":"unauthorized","message":"Key is invalid"}`)
	}))
	defer server.Close()

	mailer := NewBrevoMailer("invalid-key", "noreply@example.com", "AgentForge", server.URL, server.Client())

	err := mailer.SendRegistrationCode(context.Background(), "user@example.com", "123456")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// error 必须包含状态码与响应体，便于排查 Brevo 失败原因。
	msg := err.Error()
	if !strings.Contains(msg, "401") {
		t.Fatalf("error %q missing status 401", msg)
	}
	if !strings.Contains(msg, "Key is invalid") {
		t.Fatalf("error %q missing response body content", msg)
	}
}

func TestBrevoMailerSendsHTMLPayloadWithCode(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	mailer := NewBrevoMailer("valid-key", "noreply@example.com", "AgentForge", server.URL, server.Client())

	if err := mailer.SendRegistrationCode(context.Background(), "user@example.com", "987654"); err != nil {
		t.Fatalf("send error = %v", err)
	}
	// 必须以 HTML 发送，并突出验证码。
	if !strings.Contains(capturedBody, `"htmlContent"`) {
		t.Fatalf("payload missing htmlContent: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "987654") {
		t.Fatalf("payload missing verification code: %s", capturedBody)
	}
	// 验证码应出现在大字号样式中。
	if !strings.Contains(capturedBody, "font-size") {
		t.Fatalf("payload missing font-size styling for code: %s", capturedBody)
	}
}

func TestBrevoMailerReturnsErrorWhenServerUnreachable(t *testing.T) {
	// 启动后立即关闭服务端，使后续连接被拒绝，覆盖 httpClient.Do 错误路径。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	mailer := NewBrevoMailer("valid-key", "noreply@example.com", "AgentForge", server.URL, server.Client())
	err := mailer.SendRegistrationCode(context.Background(), "user@example.com", "123456")
	if err == nil {
		t.Fatal("expected error when server unreachable, got nil")
	}
}
