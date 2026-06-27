// Package turnstile 封装 Cloudflare Turnstile 服务端令牌校验。
// secret 为空时 NewVerifier 返回禁用态 Service，Verify 永远放行（优雅降级）。
package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var (
	ErrTokenRequired    = errors.New("turnstile token required")
	ErrTokenInvalid     = errors.New("turnstile token invalid")
	ErrActionMismatch   = errors.New("turnstile action mismatch")
	ErrHostnameMismatch = errors.New("turnstile hostname mismatch")
	ErrUnavailable      = errors.New("turnstile verify unavailable")
	ErrMisconfigured    = errors.New("turnstile misconfigured")
)

// Verifier 校验 Turnstile 令牌。DisabledVerifier 永远放行，
// 因此消费方无需感知“是否启用”，直接调用 Verify 即可。
type Verifier interface {
	Verify(ctx context.Context, token, action string) error
}

// siteVerifyResponse 对应 siteverify 返回的 JSON。
type siteVerifyResponse struct {
	Success    bool     `json:"success"`
	Action     string   `json:"action"`
	Hostname   string   `json:"hostname"`
	ErrorCodes []string `json:"error-codes"`
}

// SiteVerifyVerifier 调用 Cloudflare siteverify 校验令牌。
type SiteVerifyVerifier struct {
	secret           string
	expectedHostname string
	verifyURL        string
	httpClient       *http.Client
}

// DisabledVerifier 未配置 secret 时使用，Verify 永远放行。
type DisabledVerifier struct{}

func (DisabledVerifier) Verify(context.Context, string, string) error { return nil }

// Service 聚合 Verifier 实现与配置，供 handler 与 /api/turnstile/config 使用。
type Service struct {
	Verifier
	sitekey string
	enabled bool
}

// Enabled 反映是否启用 Turnstile（secret 已配置）。
func (s *Service) Enabled() bool { return s.enabled }

// Sitekey 返回下发给前端的 sitekey（未配置时为空）。
func (s *Service) Sitekey() string { return s.sitekey }

// NewVerifier 按 secret 是否为空返回启用或禁用态 Service。
// httpClient 为 nil 时使用带 10s 超时的默认 client（便于测试注入 fake transport）。
func NewVerifier(secret, sitekey, expectedHostname, verifyURL string, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if verifyURL == "" {
		verifyURL = defaultVerifyURL
	}
	if secret == "" {
		return &Service{Verifier: DisabledVerifier{}, sitekey: "", enabled: false}
	}
	slog.Info("Turnstile enabled")
	return &Service{
		Verifier: &SiteVerifyVerifier{
			secret:           secret,
			expectedHostname: expectedHostname,
			verifyURL:        verifyURL,
			httpClient:       httpClient,
		},
		sitekey: sitekey,
		enabled: true,
	}
}

func (v *SiteVerifyVerifier) Verify(ctx context.Context, token, action string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrTokenRequired
	}
	form := url.Values{}
	form.Set("secret", v.secret)
	form.Set("response", token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build siteverify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		// siteverify 不可达：放行优先，避免网络故障锁死登录。
		// 不在此处 log——由 requireTurnstile 在请求边界统一记录，避免重复日志。
		return fmt.Errorf("%w: siteverify request: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close() //nolint:errcheck // deferred close

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: siteverify returned status %d", ErrUnavailable, resp.StatusCode)
	}

	var result siteVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode siteverify response: %w", err)
	}

	if contains(result.ErrorCodes, "invalid-input-secret", "missing-input-secret") {
		return fmt.Errorf("%w: secret rejected by siteverify", ErrMisconfigured)
	}
	if contains(result.ErrorCodes, "internal-error") {
		return fmt.Errorf("%w: siteverify internal-error", ErrUnavailable)
	}
	if !result.Success {
		return ErrTokenInvalid
	}
	// 软校验：仅当后端期望值与 Cloudflare 返回值都非空时才比对。
	// Cloudflare 测试 sitekey 返回空 action 与 mock hostname（example.com），
	// 严格校验会让开发/测试环境所有验证失败；生产真实 sitekey 返回真实值时正常校验。
	if action != "" && result.Action != "" && result.Action != action {
		return ErrActionMismatch
	}
	if v.expectedHostname != "" && v.expectedHostname != "none" &&
		result.Hostname != "" && result.Hostname != v.expectedHostname {
		return ErrHostnameMismatch
	}
	return nil
}

// contains 报告 codes 是否包含任一 target。
func contains(codes []string, target ...string) bool {
	for _, c := range codes {
		for _, t := range target {
			if c == t {
				return true
			}
		}
	}
	return false
}
