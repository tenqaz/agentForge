package weixin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBotQRCodeReturnsPayloadAndImageContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/get_bot_qrcode" {
			t.Fatalf("path = %q, want /get_bot_qrcode", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"qrcode":"qr-payload","qrcode_img_content":"data:image/png;base64,abc"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret-api-key", server.Client())
	response, err := client.GetBotQRCode(context.Background(), QRCodeRequest{BotType: 3})
	if err != nil {
		t.Fatalf("GetBotQRCode returned error: %v", err)
	}
	if response.QRCode != "qr-payload" || response.QRCodeImageContent != "data:image/png;base64,abc" {
		t.Fatalf("response = %#v", response)
	}
}

func TestGetQRCodeStatusSwitchesBaseURLOnRedirect(t *testing.T) {
	t.Parallel()

	confirmedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/get_qrcode_status" {
			t.Fatalf("confirmed path = %q, want /get_qrcode_status", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"confirmed","ilink_bot_id":"bot-1","bot_token":"token-1","baseurl":"https://redirect.example.com","ilink_user_id":"user-1"}`))
	}))
	defer confirmedServer.Close()

	redirected := false
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected = true
		if r.URL.Path != "/get_qrcode_status" {
			t.Fatalf("redirect path = %q, want /get_qrcode_status", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"scaned_but_redirect","redirect_host":"` + confirmedServer.URL + `"}`))
	}))
	defer redirectServer.Close()

	client := NewClient(redirectServer.URL, "secret-api-key", redirectServer.Client())
	first, err := client.GetQRCodeStatus(context.Background(), QRStatusRequest{QRCode: "qr-1"})
	if err != nil {
		t.Fatalf("first GetQRCodeStatus returned error: %v", err)
	}
	if first.Status != StatusScannedButRedirect {
		t.Fatalf("first status = %q, want %q", first.Status, StatusScannedButRedirect)
	}

	second, err := client.GetQRCodeStatus(context.Background(), QRStatusRequest{QRCode: "qr-1"})
	if err != nil {
		t.Fatalf("second GetQRCodeStatus returned error: %v", err)
	}
	if !redirected {
		t.Fatal("expected initial server to receive redirecting request")
	}
	if second.Status != StatusConfirmed || second.ILinkBotID != "bot-1" || second.BotToken != "token-1" || second.BaseURL != "https://redirect.example.com" || second.ILinkUserID != "user-1" {
		t.Fatalf("second response = %#v", second)
	}
}

func TestGetQRCodeStatusConfirmedRequiresStableFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"confirmed","ilink_bot_id":"bot-1","baseurl":"https://weixin.example.com","ilink_user_id":"user-1"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret-api-key", server.Client())
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
