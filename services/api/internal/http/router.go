package http

import (
	"context"
	"net/http"

	"agentforge.local/services/api/internal/auth"
)

type AuthRepository interface {
	FindUserByEmail(ctx context.Context, email string) (auth.User, error)
	PasswordHashForUser(ctx context.Context, userID string) (string, error)
}

type Dependencies struct {
	AuthRepository AuthRepository
	SessionManager *auth.SessionManager
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	sessionHandlers := NewSessionHandlers(deps.AuthRepository, deps.SessionManager)
	mux.HandleFunc("POST /api/sessions", sessionHandlers.Create)
	mux.HandleFunc("GET /api/session", sessionHandlers.Current)
	mux.HandleFunc("DELETE /api/session", sessionHandlers.Delete)
	return mux
}
