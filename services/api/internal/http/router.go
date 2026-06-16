package http

import (
	"context"
	"net/http"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/templates"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, params auth.CreateUserParams) (auth.User, error)
	FindUserByEmail(ctx context.Context, email string) (auth.User, error)
	FindUserByID(ctx context.Context, userID string) (auth.User, error)
	PasswordHashForUser(ctx context.Context, userID string) (string, error)
}

type Dependencies struct {
	AuthRepository       AuthRepository
	SessionManager       *auth.SessionManager
	TemplateService      *templates.Service
	AgentService         *agents.Service
	RuntimeJobRepository *jobs.RuntimeRepository
	ChannelService       *channels.Service
	ChannelRepository    *channels.Repository
	ChannelJobRepository *jobs.ChannelRepository
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	NewHealthHandlers().Register(mux)
	if deps.AuthRepository != nil {
		registrationHandlers := NewRegistrationHandlers(deps.AuthRepository)
		mux.HandleFunc("POST /api/users", registrationHandlers.Create)
	}
	sessionHandlers := NewSessionHandlers(deps.AuthRepository, deps.SessionManager)
	mux.HandleFunc("POST /api/sessions", sessionHandlers.Create)
	mux.HandleFunc("GET /api/session", sessionHandlers.Current)
	mux.HandleFunc("DELETE /api/session", sessionHandlers.Delete)
	if deps.TemplateService != nil {
		templateHandlers := NewTemplateHandlers(deps.TemplateService)
		templateHandlers.Register(mux)
	}
	if deps.AgentService != nil && deps.RuntimeJobRepository != nil {
		agentHandlers := NewAgentHandlers(deps.AgentService, deps.RuntimeJobRepository)
		agentHandlers.Register(mux)
	}
	if deps.AgentService != nil && deps.ChannelService != nil && deps.ChannelRepository != nil && deps.ChannelJobRepository != nil {
		weixinHandlers := NewWeixinHandlers(deps.AgentService, deps.ChannelService, deps.ChannelRepository, deps.ChannelJobRepository)
		weixinHandlers.Register(mux)
	}
	if deps.SessionManager != nil && deps.AuthRepository != nil {
		return SessionMiddleware(deps.SessionManager, deps.AuthRepository)(mux)
	}
	return mux
}
