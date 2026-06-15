package http

import "net/http"

type HealthHandlers struct{}

func NewHealthHandlers() *HealthHandlers {
	return &HealthHandlers{}
}

func (h *HealthHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", h.Get)
}

func (h *HealthHandlers) Get(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
