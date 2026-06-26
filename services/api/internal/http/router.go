package http

import (
	"context"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/templates"
	"github.com/gin-gonic/gin"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, params auth.CreateUserParams) (auth.User, error)
	FindUserByEmail(ctx context.Context, email string) (auth.User, error)
	FindUserByID(ctx context.Context, userID string) (auth.User, error)
	PasswordHashForUser(ctx context.Context, userID string) (string, error)
}

// VerificationService 抽象验证码发码、校验与消费，由 verification.Service 实现。
type VerificationService interface {
	SendRegistrationCode(ctx context.Context, email string) error
	VerifyRegistrationCode(ctx context.Context, email, code string) error
	ConsumeRegistrationCode(ctx context.Context, email string)
}

type Dependencies struct {
	AuthRepository       AuthRepository
	SessionManager       *auth.SessionManager
	VerificationService  VerificationService
	TemplateService      *templates.Service
	AgentService         *agents.Service
	RuntimeJobRepository *jobs.RuntimeRepository
	ChannelService       *channels.Service
	ChannelRepository    *channels.Repository
	ChannelJobRepository *jobs.ChannelRepository
}

func NewRouter(deps Dependencies) *gin.Engine {
	r := gin.New()
	r.Use(RequestIDMiddleware(), RecoverMiddleware())
	if deps.SessionManager != nil && deps.AuthRepository != nil {
		r.Use(SessionMiddleware(deps.SessionManager, deps.AuthRepository))
	}

	api := r.Group("/api")
	NewHealthHandlers().Register(api)

	if deps.AuthRepository != nil && deps.VerificationService != nil {
		NewRegistrationHandlers(deps.AuthRepository, deps.VerificationService).Register(api)
	}

	NewSessionHandlers(deps.AuthRepository, deps.SessionManager).Register(api)

	if deps.TemplateService != nil {
		NewTemplateHandlers(deps.TemplateService).Register(api)
	}
	if deps.AgentService != nil && deps.RuntimeJobRepository != nil {
		NewAgentHandlers(deps.AgentService, deps.RuntimeJobRepository).Register(api)
	}
	if deps.AgentService != nil && deps.ChannelService != nil && deps.ChannelRepository != nil && deps.ChannelJobRepository != nil {
		NewWeixinHandlers(deps.AgentService, deps.ChannelService, deps.ChannelRepository, deps.ChannelJobRepository).Register(api)
	}

	return r
}
