package http

import (
	"errors"
	"net/http"

	"agentforge.local/services/api/internal/auth"
	"github.com/gin-gonic/gin"
)

type RegistrationHandlers struct {
	authRepository AuthRepository
}

func NewRegistrationHandlers(authRepository AuthRepository) *RegistrationHandlers {
	return &RegistrationHandlers{authRepository: authRepository}
}

func (h *RegistrationHandlers) Register(router gin.IRoutes) {
	router.POST("/users", h.Create)
}

func (h *RegistrationHandlers) Create(c *gin.Context) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeRequest(c, &request) {
		return
	}

	user, err := h.authRepository.CreateUser(c.Request.Context(), auth.CreateUserParams{
		Email:    request.Email,
		Password: request.Password,
		Role:     auth.RoleUser,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidEmail):
			writeError(c, http.StatusBadRequest, "invalid_email")
		case errors.Is(err, auth.ErrInvalidPassword):
			writeError(c, http.StatusBadRequest, "invalid_password")
		case errors.Is(err, auth.ErrEmailAlreadyExists):
			writeError(c, http.StatusConflict, "email_already_exists")
		case errors.Is(err, auth.ErrEmailLookupAmbiguous):
			writeError(c, http.StatusConflict, "email_conflict")
		default:
			writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		}
		return
	}

	writeJSON(c, http.StatusCreated, userResponse{User: user})
}
