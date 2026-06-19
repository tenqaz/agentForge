package weixin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the iLink Bot API endpoint, mirroring ILINK_BASE_URL
// in the Hermes weixin gateway adapter
// (gateway/platforms/weixin.py).
const DefaultBaseURL = "https://ilinkai.weixin.qq.com"

// All iLink Bot endpoints share this path prefix.
const apiPathPrefix = "/ilink/bot"

// iLink-App identity headers required on every request, mirroring
// the Hermes weixin adapter:
//
//	ILINK_APP_ID = "bot"
//	ILINK_APP_CLIENT_VERSION = (2 << 16) | (2 << 8) | 0
const (
	headerAppID          = "iLink-App-Id"
	headerAppVersion     = "iLink-App-ClientVersion"
	iLinkAppID           = "bot"
	iLinkAppVersion      = "131584" // (2<<16)|(2<<8)|0
	defaultClientTimeout = 40 * time.Second
)

const (
	StatusWait               = "wait"
	StatusScanned            = "scaned"
	StatusScannedButRedirect = "scaned_but_redirect"
	StatusExpired            = "expired"
	StatusConfirmed          = "confirmed"

	ErrCodeInvalidConfirmedResponse = "invalid_confirmed_response"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

type QRCodeRequest struct {
	BotType int `json:"bot_type"`
}

type QRCodeResponse struct {
	QRCode             string `json:"qrcode"`
	QRCodeImageContent string `json:"qrcode_img_content"`
}

type QRStatusRequest struct {
	QRCode string `json:"qrcode"`
	// BaseURLOverride, when non-empty, sends this single status request to
	// a different base URL than the client's configured default. The
	// "scaned_but_redirect" status returns a redirect_host that subsequent
	// polls in the same pairing session must use; the override is passed
	// per-call so that one pairing session's redirect cannot bleed into
	// another's (the client itself is shared across all agents).
	BaseURLOverride string `json:"-"`
}

type QRStatusResponse struct {
	Status       string `json:"status"`
	RedirectHost string `json:"redirect_host,omitempty"`
	ILinkBotID   string `json:"ilink_bot_id,omitempty"`
	BotToken     string `json:"bot_token,omitempty"`
	BaseURL      string `json:"baseurl,omitempty"`
	ILinkUserID  string `json:"ilink_user_id,omitempty"`
}

type Client interface {
	GetBotQRCode(ctx context.Context, req QRCodeRequest) (QRCodeResponse, error)
	GetQRCodeStatus(ctx context.Context, req QRStatusRequest) (QRStatusResponse, error)
}

type client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a Client for Tencent's iLink Bot API.
//
// An empty baseURL falls back to DefaultBaseURL. baseURL must be an
// absolute URL (http:// or https://); validation is the caller's job
// (config.Load enforces this for the API server).
//
// If httpClient is nil, a client with a 40s timeout (matching the Hermes
// adapter's QR_TIMEOUT_MS) is used.
//
// The QR-code endpoints are unauthenticated — the bot token only exists
// after the user confirms the scan — so this client deliberately does
// not take or send an Authorization header.
func NewClient(baseURL string, httpClient *http.Client) Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultClientTimeout}
	}
	return &client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *client) GetBotQRCode(ctx context.Context, req QRCodeRequest) (QRCodeResponse, error) {
	values := url.Values{}
	values.Set("bot_type", stringifyInt(req.BotType))

	endpoint := c.baseURL + apiPathPrefix + "/get_bot_qrcode?" + values.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return QRCodeResponse{}, err
	}
	setILinkHeaders(httpReq)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return QRCodeResponse{}, err
	}
	defer resp.Body.Close()

	var parsed QRCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return QRCodeResponse{}, err
	}
	return parsed, nil
}

func (c *client) GetQRCodeStatus(ctx context.Context, req QRStatusRequest) (QRStatusResponse, error) {
	values := url.Values{}
	values.Set("qrcode", req.QRCode)

	base := c.baseURL
	if strings.TrimSpace(req.BaseURLOverride) != "" {
		base = strings.TrimRight(req.BaseURLOverride, "/")
	}
	endpoint := base + apiPathPrefix + "/get_qrcode_status?" + values.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return QRStatusResponse{}, err
	}
	setILinkHeaders(httpReq)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return QRStatusResponse{}, err
	}
	defer resp.Body.Close()

	var parsed QRStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return QRStatusResponse{}, err
	}
	if parsed.Status == StatusConfirmed {
		if strings.TrimSpace(parsed.ILinkBotID) == "" || strings.TrimSpace(parsed.BotToken) == "" || strings.TrimSpace(parsed.BaseURL) == "" || strings.TrimSpace(parsed.ILinkUserID) == "" {
			return QRStatusResponse{}, &Error{Code: ErrCodeInvalidConfirmedResponse, Message: "confirmed response missing required fields"}
		}
	}
	return parsed, nil
}

// NormalizeRedirectHost converts a redirect_host value (which the iLink
// gateway returns as a bare host like "xx.weixin.qq.com") into a fully
// qualified base URL.
//
// Values that already include http:// or https:// are returned unchanged
// (after stripping any trailing slash) so that httptest server URLs work
// in tests without rewriting.
func NormalizeRedirectHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	host = strings.TrimRight(host, "/")
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}
	return "https://" + host
}

func setILinkHeaders(req *http.Request) {
	req.Header.Set(headerAppID, iLinkAppID)
	req.Header.Set(headerAppVersion, iLinkAppVersion)
}

func stringifyInt(value int) string {
	return fmt.Sprintf("%d", value)
}
