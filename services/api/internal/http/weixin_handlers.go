package http

import (
	"errors"
	"net/http"
	"os"
	"time"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/jobs"
)

type WeixinHandlers struct {
	agents      *agents.Service
	channels    *channels.Service
	channelRepo *channels.Repository
	channelJobs *jobs.ChannelRepository
}

func NewWeixinHandlers(agentService *agents.Service, channelService *channels.Service, channelRepo *channels.Repository, channelJobs *jobs.ChannelRepository) *WeixinHandlers {
	return &WeixinHandlers{agents: agentService, channels: channelService, channelRepo: channelRepo, channelJobs: channelJobs}
}

func (h *WeixinHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agents/{id}/channels/weixin", h.GetChannel)
	mux.HandleFunc("PUT /api/agents/{id}/channels/weixin", h.PutChannel)
	mux.HandleFunc("DELETE /api/agents/{id}/channels/weixin", h.DeleteChannel)
	mux.HandleFunc("GET /api/agents/{id}/channels/weixin/pairing-sessions", h.ListPairingSessions)
	mux.HandleFunc("POST /api/agents/{id}/channels/weixin/pairing-sessions", h.CreatePairingSession)
	mux.HandleFunc("GET /api/agents/{id}/channels/weixin/pairing-sessions/{sessionId}", h.GetPairingSession)
}

func (h *WeixinHandlers) GetChannel(w http.ResponseWriter, r *http.Request) {
	agent, ok := h.authorizeAgent(w, r)
	if !ok {
		return
	}
	channel, err := h.channelRepo.GetByAgentID(r.Context(), agent.ID)
	if errors.Is(err, channels.ErrNotFound) {
		writeJSON(w, http.StatusOK, channelResponse{Channel: channelDTO{Status: channels.StatusNotConfigured, ChannelType: channels.TypeWeixin}})
		return
	}
	if err != nil {
		writeWeixinError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, channelResponse{Channel: newChannelDTO(channel)})
}

func (h *WeixinHandlers) PutChannel(w http.ResponseWriter, r *http.Request) {
	agent, ok := h.authorizeAgent(w, r)
	if !ok {
		return
	}
	channel, err := h.channels.EnsureWeixinChannel(r.Context(), agent.ID)
	if err != nil {
		writeWeixinError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, channelResponse{Channel: newChannelDTO(channel)})
}

func (h *WeixinHandlers) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorizeAgent(w, r); !ok {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WeixinHandlers) CreatePairingSession(w http.ResponseWriter, r *http.Request) {
	agent, ok := h.authorizeAgent(w, r)
	if !ok {
		return
	}
	channel, err := h.channels.EnsureWeixinChannel(r.Context(), agent.ID)
	if err != nil {
		writeWeixinError(w, r, err)
		return
	}
	session, _, created, err := h.channelJobs.CreateOrReuseConnectJob(r.Context(), channel.ID, nowPlusFiveMinutes())
	if err != nil {
		writeWeixinError(w, r, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	writeJSON(w, status, pairingSessionResponse{Session: h.toPairingSessionDTO(session)})
}

func (h *WeixinHandlers) ListPairingSessions(w http.ResponseWriter, r *http.Request) {
	agent, ok := h.authorizeAgent(w, r)
	if !ok {
		return
	}
	channel, err := h.channelRepo.GetByAgentID(r.Context(), agent.ID)
	if err != nil {
		writeWeixinError(w, r, err)
		return
	}
	sessions, err := h.channelRepo.ListPairingSessions(r.Context(), channel.ID)
	if err != nil {
		writeWeixinError(w, r, err)
		return
	}
	result := make([]pairingSessionDTO, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, h.toPairingSessionDTO(session))
	}
	writeJSON(w, http.StatusOK, pairingSessionsResponse{Sessions: result})
}

func (h *WeixinHandlers) GetPairingSession(w http.ResponseWriter, r *http.Request) {
	agent, ok := h.authorizeAgent(w, r)
	if !ok {
		return
	}
	channel, err := h.channelRepo.GetByAgentID(r.Context(), agent.ID)
	if err != nil {
		writeWeixinError(w, r, err)
		return
	}
	session, err := h.channelRepo.GetPairingSessionByID(r.Context(), channel.ID, r.PathValue("sessionId"))
	if err != nil {
		writeWeixinError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pairingSessionResponse{Session: h.toPairingSessionDTO(session)})
}

type channelResponse struct {
	Channel channelDTO `json:"channel"`
}

type pairingSessionResponse struct {
	Session pairingSessionDTO `json:"session"`
}

type pairingSessionsResponse struct {
	Sessions []pairingSessionDTO `json:"sessions"`
}

type channelDTO struct {
	ID                string          `json:"id,omitempty"`
	AgentID           string          `json:"agentId,omitempty"`
	ChannelType       channels.Type   `json:"channelType"`
	Status            channels.Status `json:"status"`
	ExternalAccountID string          `json:"externalAccountId,omitempty"`
	LastErrorCode     string          `json:"lastErrorCode,omitempty"`
	LastErrorMessage  string          `json:"lastErrorMessage,omitempty"`
}

type pairingSessionDTO struct {
	ID             string                 `json:"id"`
	Status         channels.PairingStatus `json:"status"`
	QRPayload      string                 `json:"qrPayload,omitempty"`
	QRImageContent string                 `json:"qrImageContent,omitempty"`
	ExpiresAt      string                 `json:"expiresAt"`
}

func newChannelDTO(channel channels.Channel) channelDTO {
	return channelDTO{
		ID:                channel.ID,
		AgentID:           channel.AgentID,
		ChannelType:       channel.ChannelType,
		Status:            channel.Status,
		ExternalAccountID: channel.ExternalAccountID,
		LastErrorCode:     channel.LastErrorCode,
		LastErrorMessage:  channel.LastErrorMessage,
	}
}

func (h *WeixinHandlers) toPairingSessionDTO(session channels.PairingSession) pairingSessionDTO {
	dto := pairingSessionDTO{
		ID:        session.ID,
		Status:    session.Status,
		QRPayload: session.QRPayload,
		ExpiresAt: session.ExpiresAt,
	}
	if session.QRImagePath != "" {
		if data, err := os.ReadFile(session.QRImagePath); err == nil {
			dto.QRImageContent = string(data)
		}
	}
	return dto
}

func (h *WeixinHandlers) authorizeAgent(w http.ResponseWriter, r *http.Request) (agents.Agent, bool) {
	user, ok := requireAuthenticatedUser(w, r)
	if !ok {
		return agents.Agent{}, false
	}
	agent, err := h.agents.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAgentError(w, r, err)
		return agents.Agent{}, false
	}
	if err := auth.RequireAgentOwner(user, agent.OwnerUserID); err != nil {
		status, code, message := mapAuthzError(err)
		writeAuthError(w, status, code, message)
		return agents.Agent{}, false
	}
	return agent, true
}

func nowPlusFiveMinutes() time.Time {
	return time.Now().Add(5 * time.Minute)
}
