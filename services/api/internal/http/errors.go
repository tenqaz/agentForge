package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/templates"
)

type apiError struct {
	Code      string `json:"error"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeAPIError(w, status, code, publicMessageForCode(code), nil)
}

func writeErrorWithMsg(w http.ResponseWriter, status int, code string, msg string) {
	writeAPIError(w, status, code, msg, nil)
}

func writeInternalError(w http.ResponseWriter, r *http.Request, status int, code, message string, err error) {
	if message == "" {
		message = publicMessageForCode(code)
	}
	writeAPIError(w, status, code, message, errWithRequest(r, err))
}

func writeAPIError(w http.ResponseWriter, status int, code, message string, err error) {
	reqID := w.Header().Get("X-Request-ID")
	resp := apiError{Code: code}
	if message != "" {
		resp.Message = message
	}
	if reqID != "" {
		resp.RequestID = reqID
	}

	if err != nil {
		attrs := []any{
			"event", "api_request_failed",
			"status", status,
			"code", code,
			"request_id", reqID,
			"error", err,
		}
		if requestErr, ok := err.(*requestError); ok {
			attrs = append(attrs, "method", requestErr.Method, "path", requestErr.Path)
		}
		slog.Error("api request failed", attrs...)
	}

	writeJSON(w, status, resp)
}

func publicMessageForCode(code string) string {
	switch code {
	case "invalid_json":
		return "invalid json"
	case "invalid_credentials":
		return "invalid credentials"
	case "unauthorized":
		return "unauthorized"
	case "forbidden":
		return "forbidden"
	case "not_found":
		return "not found"
	case "conflict":
		return "conflict"
	case "invalid_request":
		return "invalid request"
	case "invalid_template":
		return "invalid template"
	case "runtime_unavailable":
		return "runtime unavailable"
	case "agent_not_running":
		return "agent not running"
	default:
		return "internal error"
	}
}

type requestError struct {
	Method string
	Path   string
	Err    error
}

func (e *requestError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return fmt.Sprintf("%s %s: %v", e.Method, e.Path, e.Err)
}

func (e *requestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func errWithRequest(r *http.Request, err error) error {
	if err == nil || r == nil {
		return err
	}
	return &requestError{
		Method: r.Method,
		Path:   r.URL.Path,
		Err:    err,
	}
}

func writeTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, templates.ErrNotFound), errors.Is(err, templates.ErrSkillNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error(), errWithRequest(r, err))
	case errors.Is(err, templates.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", err.Error(), errWithRequest(r, err))
	case errors.Is(err, templates.ErrInvalidInput):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error(), errWithRequest(r, err))
	case errors.Is(err, templates.ErrInvalidTemplate):
		writeAPIError(w, http.StatusBadRequest, "invalid_template", err.Error(), errWithRequest(r, err))
	default:
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
	}
}

func writeAgentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, agents.ErrNotFound), errors.Is(err, agents.ErrTemplateNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error(), errWithRequest(r, err))
	case errors.Is(err, agents.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", err.Error(), errWithRequest(r, err))
	case errors.Is(err, agents.ErrInvalidInput), errors.Is(err, agents.ErrInvalidStateTransition):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error(), errWithRequest(r, err))
	case errors.Is(err, agents.ErrRuntimeUnavailable):
		writeAPIError(w, http.StatusConflict, "runtime_unavailable", err.Error(), errWithRequest(r, err))
	default:
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
	}
}

func writeRuntimeJobError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, jobs.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", err.Error(), errWithRequest(r, err))
	case errors.Is(err, jobs.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", err.Error(), errWithRequest(r, err))
	case errors.Is(err, jobs.ErrInvalidInput):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error(), errWithRequest(r, err))
	case errors.Is(err, agents.ErrRuntimeUnavailable):
		writeAPIError(w, http.StatusConflict, "runtime_unavailable", err.Error(), errWithRequest(r, err))
	default:
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
	}
}

func writeWeixinError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, channels.ErrAgentNotRunning):
		writeAPIError(w, http.StatusConflict, "agent_not_running", publicMessageForCode("agent_not_running"), errWithRequest(r, err))
	case errors.Is(err, channels.ErrNotFound), errors.Is(err, jobs.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", publicMessageForCode("not_found"), errWithRequest(r, err))
	case errors.Is(err, channels.ErrConflict), errors.Is(err, jobs.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", publicMessageForCode("conflict"), errWithRequest(r, err))
	case errors.Is(err, channels.ErrInvalidInput), errors.Is(err, channels.ErrInvalidStateTransition), errors.Is(err, jobs.ErrInvalidInput):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", publicMessageForCode("invalid_request"), errWithRequest(r, err))
	default:
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
	}
}

func recoverPanic(w http.ResponseWriter, r *http.Request, recovered any) {
	slog.Error(
		"panic recovered",
		"event", "panic_recovered",
		"request_id", w.Header().Get("X-Request-ID"),
		"method", r.Method,
		"path", r.URL.Path,
		"panic", recovered,
		"stack", string(debug.Stack()),
	)
	writeAPIError(w, http.StatusInternalServerError, "internal_error", publicMessageForCode("internal_error"), nil)
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	writeAPIError(w, status, code, message, nil)
}

func mapAuthzError(err error) (int, string, string) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		return http.StatusForbidden, "forbidden", publicMessageForCode("forbidden")
	default:
		return http.StatusUnauthorized, "unauthorized", publicMessageForCode("unauthorized")
	}
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_json", publicMessageForCode("invalid_json"), errWithRequest(r, err))
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeAPIError(w, http.StatusBadRequest, "invalid_json", "extra fields in request body", errWithRequest(r, err))
		return false
	}
	return true
}
