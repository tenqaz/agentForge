package http

import (
	"errors"
	"net/http"

	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/turnstile"
	"github.com/gin-gonic/gin"
)

type SessionHandlers struct {
	authRepository AuthRepository
	sessionManager *auth.SessionManager
	turnstile      turnstile.Verifier
}

func NewSessionHandlers(authRepository AuthRepository, sessionManager *auth.SessionManager, t *turnstile.Service) *SessionHandlers {
	var v turnstile.Verifier = turnstile.DisabledVerifier{}
	if t != nil {
		v = t.Verifier
	}
	return &SessionHandlers{
		authRepository: authRepository,
		sessionManager: sessionManager,
		turnstile:      v,
	}
}

func (h *SessionHandlers) Register(router gin.IRoutes) {
	router.POST("/sessions", h.Create)
	router.GET("/session", h.Current)
	router.DELETE("/session", h.Delete)
}

func (h *SessionHandlers) Create(c *gin.Context) {
	var request struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		TurnstileToken string `json:"turnstileToken"`
	}
	if !decodeRequest(c, &request) {
		return
	}
	if !requireTurnstile(c, h.turnstile, request.TurnstileToken, "login") {
		return
	}
	user, err := h.authRepository.FindUserByEmail(c.Request.Context(), request.Email)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			auth.CheckPassword(dummyPasswordHash, request.Password)
			writeError(c, http.StatusUnauthorized, "invalid_credentials")
			return
		}
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	hash, err := h.authRepository.PasswordHashForUser(c.Request.Context(), user.ID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(c, http.StatusUnauthorized, "invalid_credentials")
			return
		}
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	if !auth.CheckPassword(hash, request.Password) {
		writeError(c, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if err := h.sessionManager.SetSessionCookie(c.Writer, user); err != nil {
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(c, http.StatusOK, userResponse{User: user})
}

func (h *SessionHandlers) Current(c *gin.Context) {
	claims, err := h.sessionManager.ParseRequest(c.Request)
	if err != nil {
		writeAuthError(c, http.StatusUnauthorized, "unauthorized", publicMessageForCode("unauthorized"))
		return
	}
	user, err := h.authRepository.FindUserByID(c.Request.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(c, http.StatusUnauthorized, "unauthorized")
			return
		}
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(c, http.StatusOK, userResponse{User: user})
}

func (h *SessionHandlers) Delete(c *gin.Context) {
	h.sessionManager.ClearSessionCookie(c.Writer)
	c.Status(http.StatusNoContent)
}

type userResponse struct {
	User auth.User `json:"user"`
}

const dummyPasswordHash = "$2a$10$7EqJtq98hPqEX7fNZaFWoOHi8a5eihfHcMN0KXpmwE5jQjlu7K.6a"

func writeJSON(c *gin.Context, status int, value any) {
	c.JSON(status, value)
}
