package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"agentforge.local/services/api/internal/auth"
)

type RegistrationHandlers struct {
	authRepository AuthRepository
}

func NewRegistrationHandlers(authRepository AuthRepository) *RegistrationHandlers {
	return &RegistrationHandlers{authRepository: authRepository}
}

func (h *RegistrationHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_json", publicMessageForCode("invalid_json"), nil)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeAPIError(w, http.StatusBadRequest, "invalid_json", "extra fields in request body", nil)
		return
	}

	user, err := h.authRepository.CreateUser(r.Context(), auth.CreateUserParams{
		Email:    request.Email,
		Password: request.Password,
		Role:     auth.RoleUser,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidEmail):
			writeError(w, http.StatusBadRequest, "invalid_email")
		case errors.Is(err, auth.ErrInvalidPassword):
			writeError(w, http.StatusBadRequest, "invalid_password")
		case errors.Is(err, auth.ErrEmailAlreadyExists):
			writeError(w, http.StatusConflict, "email_already_exists")
		case errors.Is(err, auth.ErrEmailLookupAmbiguous):
			writeError(w, http.StatusConflict, "email_conflict")
		default:
			writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
		}
		return
	}

	writeJSON(w, http.StatusCreated, userResponse{User: user})
}
