package http

import (
	"context"
	"net/http"

	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/templates"
)

type AuthRepository interface {
	FindUserByEmail(ctx context.Context, email string) (auth.User, error)
	FindUserByID(ctx context.Context, userID string) (auth.User, error)
	PasswordHashForUser(ctx context.Context, userID string) (string, error)
}

type Dependencies struct {
	AuthRepository  AuthRepository
	SessionManager  *auth.SessionManager
	TemplateService *templates.Service
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	sessionHandlers := NewSessionHandlers(deps.AuthRepository, deps.SessionManager)
	mux.HandleFunc("POST /api/sessions", sessionHandlers.Create)
	mux.HandleFunc("GET /api/session", sessionHandlers.Current)
	mux.HandleFunc("DELETE /api/session", sessionHandlers.Delete)
	if deps.TemplateService != nil {
		templateHandlers := NewTemplateHandlers(deps.TemplateService)
		templateHandlers.Register(mux)
	}
	if deps.SessionManager != nil && deps.AuthRepository != nil {
		return SessionMiddleware(deps.SessionManager, deps.AuthRepository)(mux)
	}
	return mux
}
