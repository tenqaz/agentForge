package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BrevoMailer 通过 Brevo 事务邮件 HTTP API 投递验证码邮件。
type BrevoMailer struct {
	apiKey      string
	senderEmail string
	senderName  string
	baseURL     string
	httpClient  *http.Client
}

func NewBrevoMailer(apiKey, senderEmail, senderName, baseURL string, httpClient *http.Client) *BrevoMailer {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://api.brevo.com"
	}
	return &BrevoMailer{
		apiKey:      apiKey,
		senderEmail: senderEmail,
		senderName:  senderName,
		baseURL:     strings.TrimRight(baseURL, "/"),
		httpClient:  httpClient,
	}
}

func (m *BrevoMailer) SendRegistrationCode(ctx context.Context, email, code string) error {
	payload := map[string]any{
		"sender": map[string]string{
			"email": m.senderEmail,
			"name":  m.senderName,
		},
		"to":          []map[string]string{{"email": email}},
		"subject":     "AgentForge 注册验证码",
		"htmlContent": registrationCodeEmailHTML(code),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/v3/smtp/email", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("api-key", m.apiKey)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // deferred close
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 读取并附带 Brevo 返回的响应体，便于排查认证、发件人、配额等失败原因。
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("brevo status %d (failed to read response body: %v)", resp.StatusCode, readErr)
		}
		trimmed := strings.TrimSpace(string(respBody))
		if trimmed == "" {
			return fmt.Errorf("brevo status %d", resp.StatusCode)
		}
		return fmt.Errorf("brevo status %d: %s", resp.StatusCode, trimmed)
	}
	return nil
}

// registrationCodeEmailHTML 构造注册验证码邮件的 HTML 内容：验证码大字号突出显示。
// 有效期文案由 CodeTTL 推导，避免与 service.go 中的常量脱钩。
func registrationCodeEmailHTML(code string) string {
	ttlMinutes := int(CodeTTL.Minutes())
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>AgentForge 注册验证码</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Hiragino Sans GB','Microsoft YaHei',sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f5f7;padding:32px 0;">
    <tr>
      <td align="center">
        <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,0.08);">
          <tr>
            <td style="padding:28px 40px;background-color:#1a1a2e;color:#ffffff;">
              <span style="font-size:18px;font-weight:600;letter-spacing:0.5px;">AgentForge</span>
            </td>
          </tr>
          <tr>
            <td style="padding:36px 40px 12px;">
              <h1 style="margin:0 0 8px;font-size:20px;font-weight:600;color:#111827;">注册验证码</h1>
              <p style="margin:0;font-size:15px;line-height:1.6;color:#4b5563;">
                你正在注册 AgentForge 账号。请使用下面的验证码完成注册：
              </p>
            </td>
          </tr>
          <tr>
            <td align="center" style="padding:8px 40px 28px;">
              <div style="display:inline-block;padding:20px 40px;background-color:#eef2ff;border:1px dashed #6366f1;border-radius:10px;">
                <span style="font-size:40px;font-weight:700;letter-spacing:10px;color:#4338ca;font-family:'SF Mono','JetBrains Mono',Menlo,Consolas,monospace;">%s</span>
              </div>
            </td>
          </tr>
          <tr>
            <td style="padding:0 40px 12px;">
              <p style="margin:0;font-size:14px;line-height:1.6;color:#6b7280;">
                验证码 <strong style="color:#4b5563;">%d 分钟</strong> 内有效。
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:0 40px 36px;">
              <p style="margin:0;font-size:13px;line-height:1.6;color:#9ca3af;">
                如果不是你本人操作，请忽略这封邮件，无需任何进一步操作。
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:20px 40px;background-color:#f9fafb;border-top:1px solid #f3f4f6;">
              <p style="margin:0;font-size:12px;color:#9ca3af;">
                此邮件由系统自动发送，请勿回复。© AgentForge
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, code, ttlMinutes)
}
