package weixin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	expectedQRPath     = "/ilink/bot/get_bot_qrcode"
	expectedStatusPath = "/ilink/bot/get_qrcode_status"
)

// assertILinkHeaders verifies the two iLink-App identity headers are
// present on every request, mirroring the Hermes weixin adapter's
// _api_get / _api_post helpers.
func assertILinkHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("iLink-App-Id"); got != "bot" {
		t.Fatalf("iLink-App-Id = %q, want %q", got, "bot")
	}
	if got := r.Header.Get("iLink-App-ClientVersion"); got != "131584" {
		t.Fatalf("iLink-App-ClientVersion = %q, want %q", got, "131584")
	}
	// The QR endpoints are unauthenticated — no token exists yet — so an
	// Authorization header would mean we're sending stale state.
	if got := r.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization header = %q, want empty", got)
	}
}

func TestGetBotQRCodeReturnsPayloadAndImageContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedQRPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, expectedQRPath)
		}
		if got := r.URL.Query().Get("bot_type"); got != "3" {
			t.Fatalf("bot_type = %q, want %q", got, "3")
		}
		assertILinkHeaders(t, r)
		_, _ = w.Write([]byte(`{"qrcode":"qr-payload","qrcode_img_content":"data:image/png;base64,abc"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	response, err := client.GetBotQRCode(context.Background(), QRCodeRequest{BotType: 3})
	if err != nil {
		t.Fatalf("GetBotQRCode returned error: %v", err)
	}
	if response.QRCode != "qr-payload" || response.QRCodeImageContent != "data:image/png;base64,abc" {
		t.Fatalf("response = %#v", response)
	}
}

func TestGetQRCodeStatusUsesBaseURLOverrideOnRedirect(t *testing.T) {
	t.Parallel()

	confirmedHits := 0
	confirmedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		confirmedHits++
		if r.URL.Path != expectedStatusPath {
			t.Fatalf("confirmed path = %q, want %q", r.URL.Path, expectedStatusPath)
		}
		assertILinkHeaders(t, r)
		_, _ = w.Write([]byte(`{"status":"confirmed","ilink_bot_id":"bot-1","bot_token":"token-1","baseurl":"https://redirect.example.com","ilink_user_id":"user-1"}`))
	}))
	defer confirmedServer.Close()

	redirected := false
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected = true
		if r.URL.Path != expectedStatusPath {
			t.Fatalf("redirect path = %q, want %q", r.URL.Path, expectedStatusPath)
		}
		assertILinkHeaders(t, r)
		_, _ = w.Write([]byte(`{"status":"scaned_but_redirect","redirect_host":"` + confirmedServer.URL + `"}`))
	}))
	defer redirectServer.Close()

	client := NewClient(redirectServer.URL, redirectServer.Client())
	first, err := client.GetQRCodeStatus(context.Background(), QRStatusRequest{QRCode: "qr-1"})
	if err != nil {
		t.Fatalf("first GetQRCodeStatus returned error: %v", err)
	}
	if first.Status != StatusScannedButRedirect {
		t.Fatalf("first status = %q, want %q", first.Status, StatusScannedButRedirect)
	}
	if !redirected {
		t.Fatal("expected initial server to receive redirecting request")
	}

	// Sanity check: without an override, the next call must STILL hit the
	// original (redirect) server. The client must not silently mutate its
	// base URL across calls — that would leak one pairing session's state
	// into another's.
	redirected = false
	if _, err := client.GetQRCodeStatus(context.Background(), QRStatusRequest{QRCode: "qr-1"}); err != nil {
		t.Fatalf("non-override GetQRCodeStatus returned error: %v", err)
	}
	if !redirected {
		t.Fatal("expected next call without override to hit the original server, not the redirect target")
	}
	if confirmedHits != 0 {
		t.Fatalf("confirmedHits = %d before override, want 0", confirmedHits)
	}

	// With the per-call override the request must reach the redirect target.
	override := NormalizeRedirectHost(confirmedServer.URL)
	second, err := client.GetQRCodeStatus(context.Background(), QRStatusRequest{QRCode: "qr-1", BaseURLOverride: override})
	if err != nil {
		t.Fatalf("override GetQRCodeStatus returned error: %v", err)
	}
	if confirmedHits != 1 {
		t.Fatalf("confirmedHits = %d after override, want 1", confirmedHits)
	}
	if second.Status != StatusConfirmed || second.ILinkBotID != "bot-1" || second.BotToken != "token-1" || second.BaseURL != "https://redirect.example.com" || second.ILinkUserID != "user-1" {
		t.Fatalf("second response = %#v", second)
	}
}

func TestGetQRCodeStatusConfirmedRequiresStableFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertILinkHeaders(t, r)
		_, _ = w.Write([]byte(`{"status":"confirmed","ilink_bot_id":"bot-1","baseurl":"https://weixin.example.com","ilink_user_id":"user-1"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, err := client.GetQRCodeStatus(context.Background(), QRStatusRequest{QRCode: "qr-1"})
	if err == nil {
		t.Fatal("GetQRCodeStatus error = nil, want error")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if apiErr.Code != ErrCodeInvalidConfirmedResponse {
		t.Fatalf("error code = %q, want %q", apiErr.Code, ErrCodeInvalidConfirmedResponse)
	}
}

func TestNewClientFallsBackToDefaultBaseURL(t *testing.T) {
	t.Parallel()

	c, ok := NewClient("", nil).(*client)
	if !ok {
		t.Fatal("NewClient did not return *client")
	}
	if c.baseURL != DefaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.httpClient.Timeout == 0 {
		t.Fatal("default httpClient.Timeout = 0, want non-zero (Hermes adapter uses QR_TIMEOUT_MS)")
	}
}

func TestNormalizeRedirectHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"xx.weixin.qq.com", "https://xx.weixin.qq.com"},
		{"xx.weixin.qq.com/", "https://xx.weixin.qq.com"},
		{"https://xx.weixin.qq.com", "https://xx.weixin.qq.com"},
		{"https://xx.weixin.qq.com/", "https://xx.weixin.qq.com"},
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080"},
	}
	for _, tc := range cases {
		if got := NormalizeRedirectHost(tc.in); got != tc.want {
			t.Errorf("NormalizeRedirectHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
