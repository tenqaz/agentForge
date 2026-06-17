package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"agentforge.local/services/api/internal/auth"
)

type SessionHandlers struct {
	authRepository AuthRepository
	sessionManager *auth.SessionManager
}

func NewSessionHandlers(authRepository AuthRepository, sessionManager *auth.SessionManager) *SessionHandlers {
	return &SessionHandlers{
		authRepository: authRepository,
		sessionManager: sessionManager,
	}
}

func (h *SessionHandlers) Create(w http.ResponseWriter, r *http.Request) {
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
	user, err := h.authRepository.FindUserByEmail(r.Context(), request.Email)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			auth.CheckPassword(dummyPasswordHash, request.Password)
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		}
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	hash, err := h.authRepository.PasswordHashForUser(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		}
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	if !auth.CheckPassword(hash, request.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if err := h.sessionManager.SetSessionCookie(w, user); err != nil {
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: user})
}

func (h *SessionHandlers) Current(w http.ResponseWriter, r *http.Request) {
	claims, err := h.sessionManager.ParseRequest(r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthorized", publicMessageForCode("unauthorized"))
		return
	}
	user, err := h.authRepository.FindUserByID(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: user})
}

func (h *SessionHandlers) Delete(w http.ResponseWriter, _ *http.Request) {
	h.sessionManager.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type userResponse struct {
	User auth.User `json:"user"`
}

const dummyPasswordHash = "$2a$10$7EqJtq98hPqEX7fNZaFWoOHi8a5eihfHcMN0KXpmwE5jQjlu7K.6a"

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
