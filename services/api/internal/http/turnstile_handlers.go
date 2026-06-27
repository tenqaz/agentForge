package http

import (
	"net/http"

	"agentforge.local/services/api/internal/turnstile"
	"github.com/gin-gonic/gin"
)

// TurnstileHandlers 提供前端获取 sitekey/enabled 的公开配置接口。
type TurnstileHandlers struct {
	svc *turnstile.Service
}

func NewTurnstileHandlers(svc *turnstile.Service) *TurnstileHandlers {
	return &TurnstileHandlers{svc: svc}
}

func (h *TurnstileHandlers) Register(router gin.IRoutes) {
	router.GET("/turnstile/config", h.GetConfig)
}

type turnstileConfigResponse struct {
	Sitekey string `json:"sitekey"`
	Enabled bool   `json:"enabled"`
}

// GetConfig 返回 sitekey 与 enabled，供前端决定是否渲染 widget。
// 此接口本身不接入 Turnstile 校验（配置发现接口）。
func (h *TurnstileHandlers) GetConfig(c *gin.Context) {
	writeJSON(c, http.StatusOK, turnstileConfigResponse{
		Sitekey: h.svc.Sitekey(),
		Enabled: h.svc.Enabled(),
	})
}
