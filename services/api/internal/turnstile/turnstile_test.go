package turnstile

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeTransport 拦截 siteverify 请求并返回预设响应。
type fakeTransport struct {
	statusCode int
	body       string
	gotBody    string
	gotURL     string
	err        error
}

func (t *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.gotURL = req.URL.String()
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		t.gotBody = string(b)
		req.Body.Close()
	}
	if t.err != nil {
		return nil, t.err
	}
	return &http.Response{
		StatusCode: t.statusCode,
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Header:     make(http.Header),
	}, nil
}

func newClient(t *fakeTransport) *http.Client {
	return &http.Client{Transport: t}
}

func TestDisabledVerifierWhenSecretEmpty(t *testing.T) {
	svc := NewVerifier("", "", "", "", nil)
	if svc.Enabled() {
		t.Fatalf("Enabled() = true, want false when secret empty")
	}
	if err := svc.Verify(context.Background(), "any-token", "login"); err != nil {
		t.Fatalf("disabled Verify error = %v, want nil", err)
	}
}

func TestVerifyPassesOnSuccess(t *testing.T) {
	transport := &fakeTransport{statusCode: 200, body: `{"success":true,"action":"login","hostname":"app.example.com"}`}
	svc := NewVerifier("secret", "sitekey", "app.example.com", "https://verify.example", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "login"); err != nil {
		t.Fatalf("Verify error = %v, want nil", err)
	}
	if !strings.Contains(transport.gotBody, "secret=secret") || !strings.Contains(transport.gotBody, "response=tok") {
		t.Fatalf("siteverify body = %q, want secret+response form-encoded", transport.gotBody)
	}
}

func TestVerifyTokenRequired(t *testing.T) {
	svc := NewVerifier("secret", "sitekey", "", "", nil)
	if err := svc.Verify(context.Background(), "  ", "login"); !errors.Is(err, ErrTokenRequired) {
		t.Fatalf("empty token error = %v, want ErrTokenRequired", err)
	}
}

func TestVerifyInvalidToken(t *testing.T) {
	transport := &fakeTransport{statusCode: 200, body: `{"success":false,"error-codes":["invalid-input-response"]}`}
	svc := NewVerifier("secret", "sitekey", "", "", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "login"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("error = %v, want ErrTokenInvalid", err)
	}
}

func TestVerifyActionMismatch(t *testing.T) {
	transport := &fakeTransport{statusCode: 200, body: `{"success":true,"action":"register","hostname":"app.example.com"}`}
	svc := NewVerifier("secret", "sitekey", "app.example.com", "", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "login"); !errors.Is(err, ErrActionMismatch) {
		t.Fatalf("error = %v, want ErrActionMismatch", err)
	}
}

func TestVerifyHostnameMismatch(t *testing.T) {
	transport := &fakeTransport{statusCode: 200, body: `{"success":true,"action":"login","hostname":"evil.com"}`}
	svc := NewVerifier("secret", "sitekey", "app.example.com", "", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "login"); !errors.Is(err, ErrHostnameMismatch) {
		t.Fatalf("error = %v, want ErrHostnameMismatch", err)
	}
}

func TestVerifySkipsHostnameCheckWhenEmpty(t *testing.T) {
	transport := &fakeTransport{statusCode: 200, body: `{"success":true,"action":"login","hostname":"anything"}`}
	svc := NewVerifier("secret", "sitekey", "", "", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "login"); err != nil {
		t.Fatalf("error = %v, want nil when hostname check disabled", err)
	}
}

func TestVerifyUnavailableOnNetworkError(t *testing.T) {
	transport := &fakeTransport{err: errors.New("network down")}
	svc := NewVerifier("secret", "sitekey", "", "", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "login"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestVerifyUnavailableOnInternalError(t *testing.T) {
	transport := &fakeTransport{statusCode: 200, body: `{"success":false,"error-codes":["internal-error"]}`}
	svc := NewVerifier("secret", "sitekey", "", "", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "login"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestVerifyMisconfiguredSecret(t *testing.T) {
	transport := &fakeTransport{statusCode: 200, body: `{"success":false,"error-codes":["invalid-input-secret"]}`}
	svc := NewVerifier("secret", "sitekey", "", "", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "login"); !errors.Is(err, ErrMisconfigured) {
		t.Fatalf("error = %v, want ErrMisconfigured", err)
	}
}

// 软校验：Cloudflare 测试 sitekey 返回空 action，此时跳过 action 校验。
// hostname 校验需 expectedHostname 非空且非 none 才生效；dev 用测试 sitekey 时
// 应设 EXPECTED_HOSTNAME=none 跳过（测试 sitekey 返回 mock hostname example.com）。
func TestVerifySoftCheckAllowsTestSitekeyResponse(t *testing.T) {
	transport := &fakeTransport{statusCode: 200, body: `{"success":true,"action":"","hostname":"example.com"}`}
	svc := NewVerifier("secret", "sitekey", "none", "", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "register"); err != nil {
		t.Fatalf("soft-check error = %v, want nil for test-sitekey response", err)
	}
}

// 软校验：expectedHostname 为空时跳过 hostname 校验，action 为空时跳过 action 校验。
func TestVerifySoftCheckSkipsEmptyExpectedHostname(t *testing.T) {
	transport := &fakeTransport{statusCode: 200, body: `{"success":true,"action":"","hostname":"evil.com"}`}
	svc := NewVerifier("secret", "sitekey", "", "", newClient(transport))
	if err := svc.Verify(context.Background(), "tok", "register"); err != nil {
		t.Fatalf("soft-check error = %v, want nil when expectedHostname empty", err)
	}
}
