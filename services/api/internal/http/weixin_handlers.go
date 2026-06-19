package http

import (
	"errors"
	"net/http"
	"time"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/jobs"
	"github.com/gin-gonic/gin"
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

func (h *WeixinHandlers) Register(router gin.IRoutes) {
	router.GET("/agents/:id/channels/weixin", h.GetChannel)
	router.PUT("/agents/:id/channels/weixin", h.PutChannel)
	router.DELETE("/agents/:id/channels/weixin", h.DeleteChannel)
	router.GET("/agents/:id/channels/weixin/pairing-sessions", h.ListPairingSessions)
	router.POST("/agents/:id/channels/weixin/pairing-sessions", h.CreatePairingSession)
	router.GET("/agents/:id/channels/weixin/pairing-sessions/:sessionId", h.GetPairingSession)
}

func (h *WeixinHandlers) GetChannel(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}
	channel, err := h.channelRepo.GetByAgentID(c.Request.Context(), agent.ID)
	if errors.Is(err, channels.ErrNotFound) {
		writeJSON(c, http.StatusOK, channelResponse{Channel: channelDTO{Status: channels.StatusNotConfigured, ChannelType: channels.TypeWeixin}})
		return
	}
	if err != nil {
		writeWeixinError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, channelResponse{Channel: newChannelDTO(channel)})
}

func (h *WeixinHandlers) PutChannel(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}
	channel, err := h.channels.EnsureWeixinChannel(c.Request.Context(), agent.ID)
	if err != nil {
		writeWeixinError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, channelResponse{Channel: newChannelDTO(channel)})
}

func (h *WeixinHandlers) DeleteChannel(c *gin.Context) {
	if _, ok := h.authorizeAgent(c); !ok {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WeixinHandlers) CreatePairingSession(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}
	channel, err := h.channels.EnsureWeixinChannel(c.Request.Context(), agent.ID)
	if err != nil {
		writeWeixinError(c, err)
		return
	}
	session, _, created, err := h.channelJobs.CreateOrReuseConnectJob(c.Request.Context(), channel.ID, nowPlusFiveMinutes())
	if err != nil {
		writeWeixinError(c, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	writeJSON(c, status, pairingSessionResponse{Session: h.toPairingSessionDTO(session)})
}

func (h *WeixinHandlers) ListPairingSessions(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}
	channel, err := h.channelRepo.GetByAgentID(c.Request.Context(), agent.ID)
	if err != nil {
		writeWeixinError(c, err)
		return
	}
	sessions, err := h.channelRepo.ListPairingSessions(c.Request.Context(), channel.ID)
	if err != nil {
		writeWeixinError(c, err)
		return
	}
	result := make([]pairingSessionDTO, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, h.toPairingSessionDTO(session))
	}
	writeJSON(c, http.StatusOK, pairingSessionsResponse{Sessions: result})
}

func (h *WeixinHandlers) GetPairingSession(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}
	channel, err := h.channelRepo.GetByAgentID(c.Request.Context(), agent.ID)
	if err != nil {
		writeWeixinError(c, err)
		return
	}
	session, err := h.channelRepo.GetPairingSessionByID(c.Request.Context(), channel.ID, c.Param("sessionId"))
	if err != nil {
		writeWeixinError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, pairingSessionResponse{Session: h.toPairingSessionDTO(session)})
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
	ID           string                 `json:"id"`
	Status       channels.PairingStatus `json:"status"`
	QRPayload    string                 `json:"qrPayload,omitempty"`
	// QRPayloadURL is the scannable liteapp URL (e.g.
	// https://liteapp.weixin.qq.com/q/...). The frontend must encode it
	// into a QR image client-side; it is plain text, not image data.
	QRPayloadURL string                 `json:"qrPayloadUrl,omitempty"`
	ExpiresAt    string                 `json:"expiresAt"`
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
	return pairingSessionDTO{
		ID:           session.ID,
		Status:       session.Status,
		QRPayload:    session.QRPayload,
		QRPayloadURL: session.QRPayloadURL,
		ExpiresAt:    session.ExpiresAt,
	}
}

func (h *WeixinHandlers) authorizeAgent(c *gin.Context) (agents.Agent, bool) {
	user, ok := requireAuthenticatedUser(c)
	if !ok {
		return agents.Agent{}, false
	}
	agent, err := h.agents.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeAgentError(c, err)
		return agents.Agent{}, false
	}
	if err := auth.RequireAgentOwner(user, agent.OwnerUserID); err != nil {
		status, code, message := mapAuthzError(err)
		writeAuthError(c, status, code, message)
		return agents.Agent{}, false
	}
	return agent, true
}

func nowPlusFiveMinutes() time.Time {
	return time.Now().Add(5 * time.Minute)
}
