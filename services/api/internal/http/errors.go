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
	"github.com/gin-gonic/gin"
)

type apiError struct {
	Code      string `json:"error"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func writeError(c *gin.Context, status int, code string) {
	writeAPIError(c, status, code, publicMessageForCode(code), nil)
}

func writeErrorWithMsg(c *gin.Context, status int, code, msg string) {
	writeAPIError(c, status, code, msg, nil)
}

func writeInternalError(c *gin.Context, status int, code, message string, err error) {
	if message == "" {
		message = publicMessageForCode(code)
	}
	writeAPIError(c, status, code, message, errWithRequest(c, err))
}

func requestIDFromContext(c *gin.Context) string {
	value, ok := c.Get(requestIDContextKey)
	if !ok {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}

func writeAPIError(c *gin.Context, status int, code, message string, err error) {
	reqID := requestIDFromContext(c)
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

		// 提取并添加堆栈信息
		var stackErr *errorWithStack
		if errors.As(err, &stackErr) {
			attrs = append(attrs, "stack", string(stackErr.Stack))
		} else {
			// 对于所有错误都捕获堆栈信息
			attrs = append(attrs, "stack", string(debug.Stack()))
		}

		if requestErr, ok := err.(*requestError); ok {
			attrs = append(attrs, "method", requestErr.Method, "path", requestErr.Path)
		}
		slog.Error("api request failed", attrs...)
	}

	c.JSON(status, resp)
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
	case "email_code_required":
		return "email code required"
	case "email_code_invalid":
		return "email code invalid"
	case "email_code_expired":
		return "email code expired"
	case "email_code_cooldown":
		return "email code cooldown"
	case "email_code_rate_limited":
		return "email code rate limited"
	case "email_code_attempts_exhausted":
		return "email code attempts exhausted"
	case "email_send_failed":
		return "email send failed"
	case "turnstile_required":
		return "turnstile token required"
	case "turnstile_invalid":
		return "turnstile token invalid"
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

// errorWithStack 包装错误并携带堆栈信息
type errorWithStack struct {
	//nolint:errname // internal helper type
	Err   error
	Stack []byte
}

func (e *errorWithStack) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *errorWithStack) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// withStack 为错误添加堆栈信息（如果还没有）
func withStack(err error) error {
	if err == nil {
		return nil
	}
	// 检查是否已经有堆栈
	var existing *errorWithStack
	if errors.As(err, &existing) {
		return err
	}
	return &errorWithStack{
		Err:   err,
		Stack: debug.Stack(),
	}
}

func errWithRequest(c *gin.Context, err error) error {
	if err == nil || c == nil || c.Request == nil {
		return err
	}
	// 先添加堆栈，再包装请求信息
	return &requestError{
		Method: c.Request.Method,
		Path:   c.Request.URL.Path,
		Err:    withStack(err),
	}
}

func writeTemplateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, templates.ErrNotFound), errors.Is(err, templates.ErrSkillNotFound):
		writeAPIError(c, http.StatusNotFound, "not_found", err.Error(), errWithRequest(c, err))
	case errors.Is(err, templates.ErrConflict):
		writeAPIError(c, http.StatusConflict, "conflict", err.Error(), errWithRequest(c, err))
	case errors.Is(err, templates.ErrTemplateInUse):
		writeAPIError(c, http.StatusConflict, "conflict", err.Error(), errWithRequest(c, err))
	case errors.Is(err, templates.ErrInvalidInput):
		writeAPIError(c, http.StatusBadRequest, "invalid_request", err.Error(), errWithRequest(c, err))
	case errors.Is(err, templates.ErrInvalidTemplate):
		writeAPIError(c, http.StatusBadRequest, "invalid_template", err.Error(), errWithRequest(c, err))
	default:
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
	}
}

func writeAgentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, agents.ErrNotFound), errors.Is(err, agents.ErrTemplateNotFound):
		writeAPIError(c, http.StatusNotFound, "not_found", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrConflict):
		writeAPIError(c, http.StatusConflict, "conflict", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrCannotDelete):
		writeAPIError(c, http.StatusConflict, "agent_cannot_delete", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrHasUnfinishedJobs):
		writeAPIError(c, http.StatusConflict, "agent_has_unfinished_jobs", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrInvalidInput), errors.Is(err, agents.ErrInvalidStateTransition):
		writeAPIError(c, http.StatusBadRequest, "invalid_request", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrRuntimeUnavailable):
		writeAPIError(c, http.StatusConflict, "runtime_unavailable", err.Error(), errWithRequest(c, err))
	default:
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
	}
}

func writeRuntimeJobError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, jobs.ErrNotFound):
		writeAPIError(c, http.StatusNotFound, "not_found", err.Error(), errWithRequest(c, err))
	case errors.Is(err, jobs.ErrConflict):
		writeAPIError(c, http.StatusConflict, "conflict", err.Error(), errWithRequest(c, err))
	case errors.Is(err, jobs.ErrInvalidInput):
		writeAPIError(c, http.StatusBadRequest, "invalid_request", err.Error(), errWithRequest(c, err))
	case errors.Is(err, agents.ErrRuntimeUnavailable):
		writeAPIError(c, http.StatusConflict, "runtime_unavailable", err.Error(), errWithRequest(c, err))
	default:
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
	}
}

func writeWeixinError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, channels.ErrAgentNotRunning):
		writeAPIError(c, http.StatusConflict, "agent_not_running", publicMessageForCode("agent_not_running"), errWithRequest(c, err))
	case errors.Is(err, channels.ErrNotFound), errors.Is(err, jobs.ErrNotFound):
		writeAPIError(c, http.StatusNotFound, "not_found", publicMessageForCode("not_found"), errWithRequest(c, err))
	case errors.Is(err, channels.ErrConflict), errors.Is(err, jobs.ErrConflict):
		writeAPIError(c, http.StatusConflict, "conflict", publicMessageForCode("conflict"), errWithRequest(c, err))
	case errors.Is(err, channels.ErrInvalidInput), errors.Is(err, channels.ErrInvalidStateTransition), errors.Is(err, jobs.ErrInvalidInput):
		writeAPIError(c, http.StatusBadRequest, "invalid_request", publicMessageForCode("invalid_request"), errWithRequest(c, err))
	default:
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
	}
}

func recoverPanic(c *gin.Context, recovered any) {
	slog.Error(
		"panic recovered",
		"event", "panic_recovered",
		"request_id", requestIDFromContext(c),
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"panic", recovered,
		"stack", string(debug.Stack()),
	)
	writeAPIError(c, http.StatusInternalServerError, "internal_error", publicMessageForCode("internal_error"), nil)
}

func writeAuthError(c *gin.Context, status int, code, message string) {
	writeAPIError(c, status, code, message, nil)
}

func mapAuthzError(err error) (int, string, string) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		return http.StatusForbidden, "forbidden", publicMessageForCode("forbidden")
	default:
		return http.StatusUnauthorized, "unauthorized", publicMessageForCode("unauthorized")
	}
}

func decodeRequest(c *gin.Context, target any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(target); err != nil {
		writeAPIError(c, http.StatusBadRequest, "invalid_json", publicMessageForCode("invalid_json"), errWithRequest(c, err))
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeAPIError(c, http.StatusBadRequest, "invalid_json", "extra fields in request body", errWithRequest(c, err))
		return false
	}
	return true
}
