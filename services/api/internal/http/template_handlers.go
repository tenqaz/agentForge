package http

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/templates"
	"github.com/gin-gonic/gin"
)

type TemplateHandlers struct {
	service *templates.Service
}

func NewTemplateHandlers(service *templates.Service) *TemplateHandlers {
	return &TemplateHandlers{service: service}
}

func (h *TemplateHandlers) Register(router gin.IRoutes) {
	router.GET("/templates", h.ListPublished)
	router.GET("/templates/:id", h.GetPublished)
	router.GET("/admin/templates", h.ListAdmin)
	router.GET("/admin/templates/:id", h.GetAdmin)
	router.POST("/admin/templates", h.Create)
	router.PUT("/admin/templates/:id", h.UpdateMetadata)
	router.DELETE("/admin/templates/:id", h.Delete)
	router.DELETE("/admin/templates/:id/archive", h.Archive)
	router.GET("/admin/templates/:id/soul", h.GetSoul)
	router.PUT("/admin/templates/:id/soul", h.PutSoul)
	router.GET("/admin/templates/:id/user", h.GetUser)
	router.PUT("/admin/templates/:id/user", h.PutUser)
	router.GET("/admin/templates/:id/skills", h.ListSkills)
	router.POST("/admin/templates/:id/skills", h.AddSkill)
	router.GET("/admin/templates/:id/skills/:skillId", h.GetSkill)
	router.DELETE("/admin/templates/:id/skills/:skillId", h.DeleteSkill)
	router.PUT("/admin/templates/:id/publication", h.Publish)
	router.DELETE("/admin/templates/:id/publication", h.Unpublish)
}

func (h *TemplateHandlers) ListPublished(c *gin.Context) {
	templateList, err := h.service.ListPublished(c.Request.Context())
	if err != nil {
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"templates": templateDTOs(templateList)})
}

func (h *TemplateHandlers) GetPublished(c *gin.Context) {
	template, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	if template.Status != templates.StatusPublished {
		writeErrorWithMsg(c, http.StatusNotFound, "not_found", "template not published")
		return
	}
	writeJSON(c, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) ListAdmin(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	templateList, err := h.service.ListAdmin(c.Request.Context())
	if err != nil {
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"templates": templateDTOs(templateList)})
}

func (h *TemplateHandlers) GetAdmin(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	template, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) Create(c *gin.Context) {
	user, ok := requireAdminUser(c)
	if !ok {
		return
	}

	// 先解析 multipart 表单
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		writeTemplateError(c, fmt.Errorf("%w: failed to parse multipart form: %v", templates.ErrInvalidInput, err))
		return
	}

	// 然后获取表单值
	name := c.Request.FormValue("name")
	description := c.Request.FormValue("description")
	soulContent := c.Request.FormValue("soulContent")
	userContent := c.Request.FormValue("userContent")

	var skillArchives []templates.SkillArchive
	if c.Request.MultipartForm != nil {
		files := c.Request.MultipartForm.File["skillZips"]
		for _, header := range files {
			archive, err := readMultipartSkillArchive(header)
			if err != nil {
				writeTemplateError(c, err)
				return
			}
			skillArchives = append(skillArchives, archive)
		}
	}

	template, err := h.service.CreateWithContents(c.Request.Context(), templates.CreateTemplateParams{
		CreatedBy:     user.ID,
		Name:          name,
		Description:   description,
		SoulContent:   soulContent,
		UserContent:   userContent,
		SkillArchives: skillArchives,
	})
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) UpdateMetadata(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	var request struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeRequest(c, &request) {
		return
	}
	template, err := h.service.UpdateMetadata(c.Request.Context(), c.Param("id"), request.Name, request.Description)
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) Archive(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	if _, err := h.service.Archive(c.Request.Context(), c.Param("id")); err != nil {
		writeTemplateError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TemplateHandlers) Delete(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		writeTemplateError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TemplateHandlers) GetSoul(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	content, err := h.service.Soul(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, contentResponse{Content: content})
}

func (h *TemplateHandlers) PutSoul(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	var request contentResponse
	if !decodeRequest(c, &request) {
		return
	}
	template, err := h.service.PutSoul(c.Request.Context(), c.Param("id"), request.Content)
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) GetUser(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	content, err := h.service.User(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, contentResponse{Content: content})
}

func (h *TemplateHandlers) PutUser(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	var request contentResponse
	if !decodeRequest(c, &request) {
		return
	}
	template, err := h.service.PutUser(c.Request.Context(), c.Param("id"), request.Content)
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) ListSkills(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	skills, err := h.service.ListSkills(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"skills": skillDTOs(skills)})
}

func (h *TemplateHandlers) AddSkill(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		writeErrorWithMsg(c, http.StatusBadRequest, "invalid_request", "missing or invalid 'file' field: "+err.Error())
		return
	}
	defer file.Close()
	archive, err := io.ReadAll(file)
	if err != nil {
		writeErrorWithMsg(c, http.StatusBadRequest, "invalid_request", "failed to read file: "+err.Error())
		return
	}
	skill, err := h.service.AddSkillArchive(c.Request.Context(), c.Param("id"), archive)
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, skillResponse{Skill: newSkillDTO(skill)})
}

func (h *TemplateHandlers) GetSkill(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	skill, content, err := h.service.GetSkill(c.Request.Context(), c.Param("id"), c.Param("skillId"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, skillResponse{Skill: newSkillDTO(skill), Content: content})
}

func (h *TemplateHandlers) DeleteSkill(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	result, err := h.service.DeleteSkill(c.Request.Context(), c.Param("id"), c.Param("skillId"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	if result.Cloned {
		writeJSON(c, http.StatusOK, templateResponse{Template: newTemplateDTO(result.Template)})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TemplateHandlers) Publish(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	template, err := h.service.Publish(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

func (h *TemplateHandlers) Unpublish(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}
	template, err := h.service.Unpublish(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, templateResponse{Template: newTemplateDTO(template)})
}

type templateResponse struct {
	Template templateDTO `json:"template"`
}

type contentResponse struct {
	Content string `json:"content"`
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

func requireAdminUser(c *gin.Context) (auth.User, bool) {
	user, ok := UserFromContext(c)
	if !ok {
		writeAuthError(c, http.StatusUnauthorized, "unauthorized", publicMessageForCode("unauthorized"))
		return auth.User{}, false
	}
	if err := auth.RequireAdmin(user); err != nil {
		status, code, message := mapAuthzError(err)
		writeAuthError(c, status, code, message)
		return auth.User{}, false
	}
	return user, true
}


func readMultipartSkillArchive(header *multipart.FileHeader) (templates.SkillArchive, error) {
	if header == nil {
		return templates.SkillArchive{}, fmt.Errorf("%w: missing file header", templates.ErrInvalidInput)
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
