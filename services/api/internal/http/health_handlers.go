package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandlers struct{}

func NewHealthHandlers() *HealthHandlers {
	return &HealthHandlers{}
}

func (h *HealthHandlers) Register(router gin.IRoutes) {
	router.GET("/health", h.Get)
}

func (h *HealthHandlers) Get(c *gin.Context) {
	writeJSON(c, http.StatusOK, map[string]bool{"ok": true})
}
