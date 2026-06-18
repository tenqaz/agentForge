package http

import (
	"agentforge.local/services/api/internal/auth"
	"github.com/gin-gonic/gin"
)

const userContextKey = "user"

func SessionMiddleware(manager *auth.SessionManager, authRepository AuthRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := manager.ParseRequest(c.Request)
		if err == nil {
			user, err := authRepository.FindUserByID(c.Request.Context(), claims.UserID)
			if err == nil {
				c.Set(userContextKey, user)
			}
		}
		c.Next()
	}
}

func UserFromContext(c *gin.Context) (auth.User, bool) {
	user, ok := c.Get(userContextKey)
	if !ok {
		return auth.User{}, false
	}
	typed, ok := user.(auth.User)
	return typed, ok
}
