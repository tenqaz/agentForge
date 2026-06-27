package http

import (
	"errors"
	"log/slog"
	"net/http"

	"agentforge.local/services/api/internal/turnstile"
	"github.com/gin-gonic/gin"
)

// requireTurnstile 在业务逻辑前校验 Turnstile 令牌。
// 直接调用 Verify：DisabledVerifier 永远放行；SiteVerifyVerifier 按结果分类处理。
// 返回 false 表示已写出错误响应，调用方应立即 return。
func requireTurnstile(c *gin.Context, v turnstile.Verifier, token, action string) bool {
	err := v.Verify(c.Request.Context(), token, action)
	switch {
	case err == nil:
		return true
	case errors.Is(err, turnstile.ErrUnavailable), errors.Is(err, turnstile.ErrMisconfigured):
		// siteverify 不可达或配置错误：放行优先，避免锁死登录。
		slog.Warn("turnstile verification bypassed", "error", err, "path", c.Request.URL.Path, "action", action)
		return true
	case errors.Is(err, turnstile.ErrTokenRequired):
		writeError(c, http.StatusBadRequest, "turnstile_required")
		return false
	case errors.Is(err, turnstile.ErrTokenInvalid),
		errors.Is(err, turnstile.ErrActionMismatch),
		errors.Is(err, turnstile.ErrHostnameMismatch):
		writeError(c, http.StatusBadRequest, "turnstile_invalid")
		return false
	default:
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return false
	}
}
