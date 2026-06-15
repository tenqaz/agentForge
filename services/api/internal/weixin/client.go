package weixin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	StatusWait              = "wait"
	StatusScanned           = "scaned"
	StatusScannedButRedirect = "scaned_but_redirect"
	StatusExpired           = "expired"
	StatusConfirmed         = "confirmed"

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
}

type QRStatusResponse struct {
	Status        string `json:"status"`
	RedirectHost  string `json:"redirect_host,omitempty"`
	ILinkBotID    string `json:"ilink_bot_id,omitempty"`
	BotToken      string `json:"bot_token,omitempty"`
	BaseURL       string `json:"baseurl,omitempty"`
	ILinkUserID   string `json:"ilink_user_id,omitempty"`
}

type Client interface {
	GetBotQRCode(ctx context.Context, req QRCodeRequest) (QRCodeResponse, error)
	GetQRCodeStatus(ctx context.Context, req QRStatusRequest) (QRStatusResponse, error)
}

type client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

func (c *client) GetBotQRCode(ctx context.Context, req QRCodeRequest) (QRCodeResponse, error) {
	values := url.Values{}
	values.Set("bot_type", stringifyInt(req.BotType))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/get_bot_qrcode?"+values.Encode(), nil)
	if err != nil {
		return QRCodeResponse{}, err
	}
	if strings.TrimSpace(c.apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/get_qrcode_status?"+values.Encode(), nil)
	if err != nil {
		return QRStatusResponse{}, err
	}
	if strings.TrimSpace(c.apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return QRStatusResponse{}, err
	}
	defer resp.Body.Close()

	var parsed QRStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return QRStatusResponse{}, err
	}
	if parsed.Status == StatusScannedButRedirect && strings.TrimSpace(parsed.RedirectHost) != "" {
		c.baseURL = strings.TrimRight(parsed.RedirectHost, "/")
	}
	if parsed.Status == StatusConfirmed {
		if strings.TrimSpace(parsed.ILinkBotID) == "" || strings.TrimSpace(parsed.BotToken) == "" || strings.TrimSpace(parsed.BaseURL) == "" || strings.TrimSpace(parsed.ILinkUserID) == "" {
			return QRStatusResponse{}, &Error{Code: ErrCodeInvalidConfirmedResponse, Message: "confirmed response missing required fields"}
		}
	}
	return parsed, nil
}

func stringifyInt(value int) string {
	return fmt.Sprintf("%d", value)
}
