package http

import (
	"fmt"
	"io"
	"mime/multipart"
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
	mux.HandleFunc("GET /api/admin/templates", h.ListAdmin)
	mux.HandleFunc("GET /api/admin/templates/{id}", h.GetAdmin)
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
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": templateDTOs(templateList)})
}

func (h *TemplateHandlers) GetPublished(w http.ResponseWriter, r *http.Request) {
	template, err := h.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, r, err)
		return
	}
	if template.Status != templates.StatusPublished {
		writeErrorWithMsg(w, http.StatusNotFound, "not_found", "template not published")
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) ListAdmin(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	templateList, err := h.service.ListAdmin(r.Context())
	if err != nil {
		writeInternalError(w, r, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": templateDTOs(templateList)})
}

func (h *TemplateHandlers) GetAdmin(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	template, err := h.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := requireAdminUser(w, r)
	if !ok {
		return
	}
	request, err := decodeTemplateCreateRequest(r)
	if err != nil {
		writeTemplateError(w, r, err)
		return
	}
	template, err := h.service.CreateWithContents(r.Context(), templates.CreateTemplateParams{
		CreatedBy:     user.ID,
		Name:          request.Name,
		Description:   request.Description,
		SoulContent:   request.SoulContent,
		UserContent:   request.UserContent,
		SkillArchives: request.SkillArchives,
	})
	if err != nil {
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, templateResponse{Template: newTemplateDTO(template)})
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
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) Archive(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	if _, err := h.service.Archive(r.Context(), r.PathValue("id")); err != nil {
		writeTemplateError(w, r, err)
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
		writeTemplateError(w, r, err)
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
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) GetUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	content, err := h.service.User(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, r, err)
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
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) ListSkills(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	skills, err := h.service.ListSkills(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skillDTOs(skills)})
}

func (h *TemplateHandlers) AddSkill(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeErrorWithMsg(w, http.StatusBadRequest, "invalid_request", "failed to parse multipart form: "+err.Error())
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErrorWithMsg(w, http.StatusBadRequest, "invalid_request", "missing or invalid 'file' field: "+err.Error())
		return
	}
	defer file.Close()
	archive, err := io.ReadAll(file)
	if err != nil {
		writeErrorWithMsg(w, http.StatusBadRequest, "invalid_request", "failed to read file: "+err.Error())
		return
	}
	skill, err := h.service.AddSkillArchive(r.Context(), r.PathValue("id"), archive)
	if err != nil {
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, skillResponse{Skill: newSkillDTO(skill)})
}

func (h *TemplateHandlers) GetSkill(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	skill, content, err := h.service.GetSkill(r.Context(), r.PathValue("id"), r.PathValue("skillId"))
	if err != nil {
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, skillResponse{Skill: newSkillDTO(skill), Content: content})
}

func (h *TemplateHandlers) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	result, err := h.service.DeleteSkill(r.Context(), r.PathValue("id"), r.PathValue("skillId"))
	if err != nil {
		writeTemplateError(w, r, err)
		return
	}
	if result.Cloned {
		writeJSON(w, http.StatusOK, templateResponse{Template: newTemplateDTO(result.Template)})
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
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) Unpublish(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminUser(w, r); !ok {
		return
	}
	template, err := h.service.Unpublish(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTemplateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

type templateResponse struct {
	Template templateDTO `json:"template"`
}

type contentResponse struct {
	Content string `json:"content"`
}

type createTemplateRequest struct {
	Name          string
	Description   string
	SoulContent   string
	UserContent   string
	SkillArchives []templates.SkillArchive
}

type skillResponse struct {
	Skill   skillDTO `json:"skill"`
	Content string   `json:"content,omitempty"`
}

type templateDTO struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Status      templates.Status `json:"status"`
	Version     int              `json:"version"`
	CreatedAt   string           `json:"createdAt"`
	UpdatedAt   string           `json:"updatedAt"`
	PublishedAt *string          `json:"publishedAt,omitempty"`
}

func newTemplateDTO(template templates.Template) templateDTO {
	return templateDTO{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		Status:      template.Status,
		Version:     template.Version,
		CreatedAt:   template.CreatedAt,
		UpdatedAt:   template.UpdatedAt,
		PublishedAt: template.PublishedAt,
	}
}

func templateDTOs(templateList []templates.Template) []templateDTO {
	result := make([]templateDTO, 0, len(templateList))
	for _, template := range templateList {
		result = append(result, newTemplateDTO(template))
	}
	return result
}

type skillDTO struct {
	ID         string `json:"id"`
	TemplateID string `json:"templateId"`
	SkillName  string `json:"skillName"`
	Checksum   string `json:"checksum"`
	CreatedAt  string `json:"createdAt"`
}

func newSkillDTO(skill templates.Skill) skillDTO {
	return skillDTO{
		ID:         skill.ID,
		TemplateID: skill.TemplateID,
		SkillName:  skill.SkillName,
		Checksum:   skill.Checksum,
		CreatedAt:  skill.CreatedAt,
	}
}

func skillDTOs(skills []templates.Skill) []skillDTO {
	result := make([]skillDTO, 0, len(skills))
	for _, skill := range skills {
		result = append(result, newSkillDTO(skill))
	}
	return result
}

func requireAdminUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "unauthorized", publicMessageForCode("unauthorized"))
		return auth.User{}, false
	}
	if err := auth.RequireAdmin(user); err != nil {
		status, code, message := mapAuthzError(err)
		writeAuthError(w, status, code, message)
		return auth.User{}, false
	}
	return user, true
}


func decodeTemplateCreateRequest(r *http.Request) (createTemplateRequest, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return createTemplateRequest{}, fmt.Errorf("%w: failed to parse multipart form: %v", templates.ErrInvalidInput, err)
	}
	request := createTemplateRequest{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		SoulContent: r.FormValue("soulContent"),
		UserContent: r.FormValue("userContent"),
	}
	if r.MultipartForm == nil {
		return request, nil
	}
	files := r.MultipartForm.File["skillZips"]
	for _, header := range files {
		archive, err := readMultipartSkillArchive(header)
		if err != nil {
			return createTemplateRequest{}, err
		}
		request.SkillArchives = append(request.SkillArchives, archive)
	}
	return request, nil
}

func readMultipartSkillArchive(header *multipart.FileHeader) (templates.SkillArchive, error) {
	if header == nil {
		return templates.SkillArchive{}, templates.ErrInvalidInput
	}
	file, err := header.Open()
	if err != nil {
		return templates.SkillArchive{}, fmt.Errorf("%w: failed to open skill file: %v", templates.ErrInvalidInput, err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return templates.SkillArchive{}, fmt.Errorf("%w: failed to read skill file: %v", templates.ErrInvalidInput, err)
	}
	return templates.SkillArchive{
		Filename: header.Filename,
		Content:  content,
	}, nil
}
