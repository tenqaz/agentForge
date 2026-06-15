package http

import (
	"context"
	"net/http"

	"agentforge.local/services/api/internal/auth"
)

type contextKey string

const userContextKey contextKey = "user"

func SessionMiddleware(manager *auth.SessionManager, authRepository AuthRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := manager.ParseRequest(r)
			if err == nil {
				user, err := authRepository.FindUserByID(r.Context(), claims.UserID)
				if err == nil {
					r = r.WithContext(context.WithValue(r.Context(), userContextKey, user))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func UserFromContext(ctx context.Context) (auth.User, bool) {
	user, ok := ctx.Value(userContextKey).(auth.User)
	return user, ok
}
