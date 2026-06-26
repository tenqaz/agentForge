package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/verification"
	"github.com/gin-gonic/gin"
)

type RegistrationHandlers struct {
	authRepository      AuthRepository
	verificationService VerificationService
}

func NewRegistrationHandlers(authRepository AuthRepository, verificationService VerificationService) *RegistrationHandlers {
	return &RegistrationHandlers{authRepository: authRepository, verificationService: verificationService}
}

// normalizeEmail 去除首尾空白并转为小写，保证发码查重、校验、建号、消费使用同一邮箱键。
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (h *RegistrationHandlers) Register(router gin.IRoutes) {
	router.POST("/users", h.Create)
	router.POST("/registration/email-codes", h.SendEmailCode)
}

// SendEmailCode 对规范化后的邮箱做已注册查重，再调用验证码服务发码。
// 已注册邮箱直接返回冲突，避免浪费发信额度。
func (h *RegistrationHandlers) SendEmailCode(c *gin.Context) {
	var request struct {
		Email string `json:"email"`
	}
	if !decodeRequest(c, &request) {
		return
	}
	normalized := normalizeEmail(request.Email)
	if normalized == "" {
		writeError(c, http.StatusBadRequest, "invalid_email")
		return
	}
	if _, err := h.authRepository.FindUserByEmail(c.Request.Context(), normalized); err == nil {
		writeError(c, http.StatusConflict, "email_already_exists")
		return
	} else if !errors.Is(err, auth.ErrUserNotFound) {
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	if err := h.verificationService.SendRegistrationCode(c.Request.Context(), normalized); err != nil {
		if retryAfter := verification.RetryAfterSeconds(err); retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		switch {
		case errors.Is(err, verification.ErrInvalidEmail):
			writeError(c, http.StatusBadRequest, "invalid_email")
		case errors.Is(err, verification.ErrCodeCooldown):
			writeError(c, http.StatusTooManyRequests, "email_code_cooldown")
		case errors.Is(err, verification.ErrCodeRateLimited):
			writeError(c, http.StatusTooManyRequests, "email_code_rate_limited")
		case errors.Is(err, verification.ErrEmailSendFailed):
			// 保留底层错误（如 Brevo 状态码/网络错误）写入日志，便于排查发信失败原因；
			// 对客户端仍只暴露稳定的 email_send_failed 错误码。
			writeInternalError(c, http.StatusInternalServerError, "email_send_failed", "", err)
		default:
			writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		}
		return
	}
	writeJSON(c, http.StatusAccepted, map[string]bool{"ok": true})
}

// Create 先校验验证码（不消费），再创建用户；仅当用户创建成功后才消费验证码，
// 避免创建失败时误损失验证码。
func (h *RegistrationHandlers) Create(c *gin.Context) {
	var request struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		EmailCode string `json:"emailCode"`
	}
	if !decodeRequest(c, &request) {
		return
	}
	if strings.TrimSpace(request.EmailCode) == "" {
		writeError(c, http.StatusBadRequest, "email_code_required")
		return
	}
	// 全程使用归一化邮箱，保证发码、校验、建号、消费键一致，避免存储的身份与
	// 验证键脱钩（如 " User@Example.com " 与 "user@example.com" 被当作不同账户）。
	normalized := normalizeEmail(request.Email)
	if err := h.verificationService.VerifyRegistrationCode(c.Request.Context(), normalized, request.EmailCode); err != nil {
		switch {
		case errors.Is(err, verification.ErrCodeExpired):
			writeError(c, http.StatusBadRequest, "email_code_expired")
		case errors.Is(err, verification.ErrCodeAttemptsExhausted):
			writeError(c, http.StatusBadRequest, "email_code_attempts_exhausted")
		case errors.Is(err, verification.ErrCodeInvalid):
			writeError(c, http.StatusBadRequest, "email_code_invalid")
		default:
			writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		}
		return
	}

	user, err := h.authRepository.CreateUser(c.Request.Context(), auth.CreateUserParams{
		Email:    normalized,
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

	h.verificationService.ConsumeRegistrationCode(c.Request.Context(), normalized)
	writeJSON(c, http.StatusCreated, userResponse{User: user})
}
