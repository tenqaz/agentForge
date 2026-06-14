package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/templates"
)

type TemplateHandlers struct {
	service *templates.Service
}

func NewTemplateHandlers(service *templates.Service) *TemplateHandlers {
	return &TemplateHandlers{service: service}
}

func (h *TemplateHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/templates", h.ListPublished)
	mux.HandleFunc("GET /api/templates/{id}", h.GetPublished)
	mux.HandleFunc("POST /api/admin/templates", h.Create)
	mux.HandleFunc("PUT /api/admin/templates/{id}", h.UpdateMetadata)
	mux.HandleFunc("DELETE /api/admin/templates/{id}", h.Archive)
	mux.HandleFunc("GET /api/admin/templates/{id}/soul", h.GetSoul)
	mux.HandleFunc("PUT /api/admin/templates/{id}/soul", h.PutSoul)
	mux.HandleFunc("GET /api/admin/templates/{id}/user", h.GetUser)
	mux.HandleFunc("PUT /api/admin/templates/{id}/user", h.PutUser)
	mux.HandleFunc("GET /api/admin/templates/{id}/skills", h.ListSkills)
	mux.HandleFunc("POST /api/admin/templates/{id}/skills", h.AddSkill)
	mux.HandleFunc("GET /api/admin/templates/{id}/skills/{skillId}", h.GetSkill)
	mux.HandleFunc("DELETE /api/admin/templates/{id}/skills/{skillId}", h.DeleteSkill)
	mux.HandleFunc("PUT /api/admin/templates/{id}/publication", h.Publish)
	mux.HandleFunc("DELETE /api/admin/templates/{id}/publication", h.Unpublish)
}

func (h *TemplateHandlers) ListPublished(w http.ResponseWriter, r *http.Request) {
	templateList, err := h.service.ListPublished(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": templateList})
}

func (h *TemplateHandlers) GetPublished(w http.ResponseWriter, r *http.Request) {
	template, err := h.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	if template.Status != templates.StatusPublished {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: template})
}

func (h *TemplateHandlers) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := requireAdminUser(w, r)
	if !ok {
		return
	}
	var request struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	template, err := h.service.Create(r.Context(), user.ID, request.Name, request.Description)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, templateResponse{Template: template})
}

func (h *TemplateHandlers) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	var request struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	template, err := h.service.UpdateMetadata(r.Context(), r.PathValue("id"), request.Name, request.Description)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: template})
}

func (h *TemplateHandlers) Archive(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	if _, err := h.service.Archive(r.Context(), r.PathValue("id")); err != nil {
		writeTemplateError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateHandlers) GetSoul(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	content, err := h.service.Soul(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contentResponse{Content: content})
}

func (h *TemplateHandlers) PutSoul(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	var request contentResponse
	if !decodeRequest(w, r, &request) {
		return
	}
	template, err := h.service.PutSoul(r.Context(), r.PathValue("id"), request.Content)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: template})
}

func (h *TemplateHandlers) GetUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	content, err := h.service.User(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contentResponse{Content: content})
}

func (h *TemplateHandlers) PutUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	var request contentResponse
	if !decodeRequest(w, r, &request) {
		return
	}
	template, err := h.service.PutUser(r.Context(), r.PathValue("id"), request.Content)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: template})
}

func (h *TemplateHandlers) ListSkills(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	skills, err := h.service.ListSkills(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skills})
}

func (h *TemplateHandlers) AddSkill(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	var request struct {
		SkillName string `json:"skillName"`
		SkillMD   string `json:"skillMD"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	skill, err := h.service.AddSkill(r.Context(), r.PathValue("id"), request.SkillName, request.SkillMD)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, skillResponse{Skill: skill})
}

func (h *TemplateHandlers) GetSkill(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	skill, content, err := h.service.GetSkill(r.Context(), r.PathValue("id"), r.PathValue("skillId"))
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skillResponse{Skill: skill, Content: content})
}

func (h *TemplateHandlers) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	if err := h.service.DeleteSkill(r.Context(), r.PathValue("id"), r.PathValue("skillId")); err != nil {
		writeTemplateError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateHandlers) Publish(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	template, err := h.service.Publish(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: template})
}

func (h *TemplateHandlers) Unpublish(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	template, err := h.service.Unpublish(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: template})
}

type templateResponse struct {
	Template templates.Template `json:"template"`
}

type contentResponse struct {
	Content string `json:"content"`
}

type skillResponse struct {
	Skill   templates.Skill `json:"skill"`
	Content string          `json:"content,omitempty"`
}

func requireAdminUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return auth.User{}, false
	}
	if err := auth.RequireAdmin(user); err != nil {
		writeError(w, http.StatusForbidden, "forbidden")
		return auth.User{}, false
	}
	return user, true
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func writeTemplateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, templates.ErrNotFound), errors.Is(err, templates.ErrSkillNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, templates.ErrConflict):
		writeError(w, http.StatusConflict, "conflict")
	case errors.Is(err, templates.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, templates.ErrInvalidTemplate):
		writeError(w, http.StatusBadRequest, "invalid_template")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}
