package http

import (
	"net/http"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/jobs"
	"github.com/gin-gonic/gin"
)

type AgentHandlers struct {
	service     *agents.Service
	runtimeJobs *jobs.RuntimeRepository
}

func NewAgentHandlers(service *agents.Service, runtimeJobs *jobs.RuntimeRepository) *AgentHandlers {
	return &AgentHandlers{service: service, runtimeJobs: runtimeJobs}
}

func (h *AgentHandlers) Register(router gin.IRoutes) {
	router.POST("/agents", h.Create)
	router.GET("/agents", h.List)
	router.GET("/agents/:id", h.Get)
	router.GET("/agents/:id/runtime", h.GetRuntime)
	router.GET("/agents/:id/runtime-jobs", h.ListRuntimeJobs)
	router.POST("/agents/:id/runtime-jobs", h.CreateRuntimeJob)
	router.GET("/agents/:id/runtime-jobs/:jobId", h.GetRuntimeJob)
}

func (h *AgentHandlers) Create(c *gin.Context) {
	user, ok := requireAuthenticatedUser(c)
	if !ok {
		return
	}

	var request struct {
		TemplateID string `json:"templateId"`
		Name       string `json:"name"`
	}
	if !decodeRequest(c, &request) {
		return
	}

	agent, err := h.service.Create(c.Request.Context(), agents.CreateParams{
		OwnerUserID: user.ID,
		TemplateID:  request.TemplateID,
		Name:        request.Name,
	})
	if err != nil {
		writeAgentError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, agentResponse{Agent: newAgentDTO(agent)})
}

func (h *AgentHandlers) List(c *gin.Context) {
	user, ok := requireAuthenticatedUser(c)
	if !ok {
		return
	}

	var (
		agentList []agents.Agent
		err       error
	)
	if user.Role == auth.RoleAdmin {
		agentList, err = h.service.List(c.Request.Context())
	} else {
		agentList, err = h.service.ListByOwner(c.Request.Context(), user.ID)
	}
	if err != nil {
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"agents": agentDTOs(agentList)})
}

func (h *AgentHandlers) Get(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}
	writeJSON(c, http.StatusOK, agentResponse{Agent: newAgentDTO(agent)})
}

func (h *AgentHandlers) GetRuntime(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}
	runtime, err := h.service.Runtime(c.Request.Context(), agent.ID)
	if err != nil {
		writeAgentError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, runtimeResponse{Runtime: newAgentRuntimeDTO(runtime)})
}

func (h *AgentHandlers) ListRuntimeJobs(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}
	runtimeJobs, err := h.runtimeJobs.ListByAgent(c.Request.Context(), agent.ID)
	if err != nil {
		writeInternalError(c, http.StatusInternalServerError, "internal_error", "", err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"jobs": runtimeJobDTOs(runtimeJobs)})
}

func (h *AgentHandlers) CreateRuntimeJob(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}

	var request struct {
		Type jobs.Type `json:"type"`
	}
	if !decodeRequest(c, &request) {
		return
	}
	if request.Type != jobs.TypeRestartRuntime {
		writeErrorWithMsg(c, http.StatusBadRequest, "invalid_request", "only restart_runtime type is supported")
		return
	}

	job, err := h.service.CreateRuntimeJob(c.Request.Context(), agent.ID, request.Type)
	if err != nil {
		writeRuntimeJobError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, runtimeJobResponse{Job: newRuntimeJobDTO(job)})
}

func (h *AgentHandlers) GetRuntimeJob(c *gin.Context) {
	agent, ok := h.authorizeAgent(c)
	if !ok {
		return
	}
	job, err := h.runtimeJobs.GetByID(c.Request.Context(), agent.ID, c.Param("jobId"))
	if err != nil {
		writeRuntimeJobError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, runtimeJobResponse{Job: newRuntimeJobDTO(job)})
}

type agentResponse struct {
	Agent agentDTO `json:"agent"`
}

type runtimeResponse struct {
	Runtime agentRuntimeDTO `json:"runtime"`
}

type runtimeJobResponse struct {
	Job runtimeJobDTO `json:"job"`
}

type agentDTO struct {
	ID               string        `json:"id"`
	OwnerUserID      string        `json:"ownerUserId"`
	TemplateID       string        `json:"templateId"`
	TemplateVersion  int           `json:"templateVersion"`
	Name             string        `json:"name"`
	Status           agents.Status `json:"status"`
	RuntimeID        string        `json:"runtimeId"`
	LastErrorCode    string        `json:"lastErrorCode"`
	LastErrorMessage string        `json:"lastErrorMessage"`
	CreatedAt        string        `json:"createdAt"`
	UpdatedAt        string        `json:"updatedAt"`
}

func newAgentDTO(agent agents.Agent) agentDTO {
	return agentDTO{
		ID:               agent.ID,
		OwnerUserID:      agent.OwnerUserID,
		TemplateID:       agent.TemplateID,
		TemplateVersion:  agent.TemplateVersion,
		Name:             agent.Name,
		Status:           agent.Status,
		RuntimeID:        agent.RuntimeID,
		LastErrorCode:    agent.LastErrorCode,
		LastErrorMessage: agent.LastErrorMessage,
		CreatedAt:        agent.CreatedAt,
		UpdatedAt:        agent.UpdatedAt,
	}
}

func agentDTOs(agentsList []agents.Agent) []agentDTO {
	result := make([]agentDTO, 0, len(agentsList))
	for _, agent := range agentsList {
		result = append(result, newAgentDTO(agent))
	}
	return result
}

type agentRuntimeDTO struct {
	AgentID          string        `json:"agentId"`
	RuntimeID        string        `json:"runtimeId"`
	Status           agents.Status `json:"status"`
	LastErrorCode    string        `json:"lastErrorCode"`
	LastErrorMessage string        `json:"lastErrorMessage"`
	UpdatedAt        string        `json:"updatedAt"`
}

func newAgentRuntimeDTO(runtime agents.Runtime) agentRuntimeDTO {
	return agentRuntimeDTO{
		AgentID:          runtime.AgentID,
		RuntimeID:        runtime.RuntimeID,
		Status:           runtime.Status,
		LastErrorCode:    runtime.LastErrorCode,
		LastErrorMessage: runtime.LastErrorMessage,
		UpdatedAt:        runtime.UpdatedAt,
	}
}

type runtimeJobDTO struct {
	ID               string      `json:"id"`
	AgentID          string      `json:"agentId"`
	Type             jobs.Type   `json:"type"`
	Status           jobs.Status `json:"status"`
	Priority         int         `json:"priority"`
	AttemptCount     int         `json:"attemptCount"`
	MaxAttempts      int         `json:"maxAttempts"`
	LockedUntil      *string     `json:"lockedUntil,omitempty"`
	LastErrorCode    string      `json:"lastErrorCode"`
	LastErrorMessage string      `json:"lastErrorMessage"`
	CreatedAt        string      `json:"createdAt"`
	UpdatedAt        string      `json:"updatedAt"`
	StartedAt        *string     `json:"startedAt,omitempty"`
	FinishedAt       *string     `json:"finishedAt,omitempty"`
}

func newRuntimeJobDTO(job jobs.RuntimeJob) runtimeJobDTO {
	return runtimeJobDTO{
		ID:               job.ID,
		AgentID:          job.AgentID,
		Type:             job.Type,
		Status:           job.Status,
		Priority:         job.Priority,
		AttemptCount:     job.AttemptCount,
		MaxAttempts:      job.MaxAttempts,
		LockedUntil:      job.LockedUntil,
		LastErrorCode:    job.LastErrorCode,
		LastErrorMessage: job.LastErrorMessage,
		CreatedAt:        job.CreatedAt,
		UpdatedAt:        job.UpdatedAt,
		StartedAt:        job.StartedAt,
		FinishedAt:       job.FinishedAt,
	}
}

func runtimeJobDTOs(jobsList []jobs.RuntimeJob) []runtimeJobDTO {
	result := make([]runtimeJobDTO, 0, len(jobsList))
	for _, job := range jobsList {
		result = append(result, newRuntimeJobDTO(job))
	}
	return result
}

func (h *AgentHandlers) authorizeAgent(c *gin.Context) (agents.Agent, bool) {
	user, ok := requireAuthenticatedUser(c)
	if !ok {
		return agents.Agent{}, false
	}
	agent, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeAgentError(c, err)
		return agents.Agent{}, false
	}
	if err := auth.RequireAgentOwner(user, agent.OwnerUserID); err != nil {
		status, code, message := mapAuthzError(err)
		writeAuthError(c, status, code, message)
		return agents.Agent{}, false
	}
	return agent, true
}

func requireAuthenticatedUser(c *gin.Context) (auth.User, bool) {
	user, ok := UserFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return auth.User{}, false
	}
	return user, true
}
