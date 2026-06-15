package channels

import "errors"

type Type string
type Status string
type PairingStatus string

const (
	TypeWeixin Type = "weixin"

	StatusNotConfigured Status = "not_configured"
	StatusQRPending     Status = "qr_pending"
	StatusConnected     Status = "connected"
	StatusError         Status = "error"
	StatusDisconnected  Status = "disconnected"

	PairingStatusPending   PairingStatus = "pending"
	PairingStatusConnected PairingStatus = "connected"
	PairingStatusExpired   PairingStatus = "expired"
	PairingStatusFailed    PairingStatus = "failed"
)

var (
	ErrNotFound               = errors.New("channel not found")
	ErrConflict               = errors.New("channel conflict")
	ErrInvalidInput           = errors.New("invalid channel input")
	ErrInvalidStateTransition = errors.New("invalid channel status transition")
	ErrAgentNotRunning        = errors.New("agent not running")
)

type Channel struct {
	ID               string `json:"id"`
	AgentID          string `json:"agentId"`
	ChannelType      Type   `json:"channelType"`
	Status           Status `json:"status"`
	ExternalAccountID string `json:"externalAccountId"`
	LastErrorCode    string `json:"lastErrorCode"`
	LastErrorMessage string `json:"lastErrorMessage"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type PairingSession struct {
	ID               string        `json:"id"`
	AgentChannelID   string        `json:"agentChannelId"`
	Status           PairingStatus `json:"status"`
	QRPayload        string        `json:"qrPayload"`
	QRImagePath      string        `json:"qrImagePath"`
	ExpiresAt        string        `json:"expiresAt"`
	AttemptCount     int           `json:"attemptCount"`
	LastErrorCode    string        `json:"lastErrorCode"`
	LastErrorMessage string        `json:"lastErrorMessage"`
	CreatedAt        string        `json:"createdAt"`
	UpdatedAt        string        `json:"updatedAt"`
}

var transitions = map[Status]map[Status]struct{}{
	StatusNotConfigured: {StatusQRPending: {}},
	StatusQRPending:     {StatusConnected: {}, StatusError: {}, StatusNotConfigured: {}},
	StatusConnected:     {StatusDisconnected: {}},
	StatusDisconnected:  {StatusQRPending: {}},
	StatusError:         {StatusQRPending: {}},
}

func (s Status) CanTransitionTo(next Status) bool {
	nextStates, ok := transitions[s]
	if !ok {
		return false
	}
	_, ok = nextStates[next]
	return ok
}
