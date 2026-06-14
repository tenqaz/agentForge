package http

import (
	"encoding/json"
	"errors"
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
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	user, err := h.authRepository.FindUserByEmail(r.Context(), request.Email)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	hash, err := h.authRepository.PasswordHashForUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if !auth.CheckPassword(hash, request.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if err := h.sessionManager.SetSessionCookie(w, user); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: user})
}

func (h *SessionHandlers) Current(w http.ResponseWriter, r *http.Request) {
	claims, err := h.sessionManager.ParseRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: claims.User})
}

func (h *SessionHandlers) Delete(w http.ResponseWriter, _ *http.Request) {
	h.sessionManager.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type userResponse struct {
	User auth.User `json:"user"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
