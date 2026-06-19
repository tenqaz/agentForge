package agents

import "errors"

type Status string

const (
	StatusCreating     Status = "creating"
	StatusProvisioning Status = "provisioning"
	StatusStarting     Status = "starting"
	StatusRunning      Status = "running"
	StatusStopped      Status = "stopped"
	StatusError        Status = "error"
)

var (
	ErrNotFound               = errors.New("agent not found")
	ErrTemplateNotFound       = errors.New("agent template not found")
	ErrConflict               = errors.New("agent conflict")
	ErrInvalidInput           = errors.New("invalid agent input")
	ErrInvalidStateTransition = errors.New("invalid agent status transition")
	ErrRuntimeUnavailable     = errors.New("agent runtime unavailable")
	ErrCannotDelete           = errors.New("agent cannot be deleted in current state")
	ErrHasUnfinishedJobs      = errors.New("agent has unfinished runtime jobs")
)

type Agent struct {
	ID               string `json:"id"`
	OwnerUserID      string `json:"ownerUserId"`
	TemplateID       string `json:"templateId"`
	TemplateVersion  int    `json:"templateVersion"`
	Name             string `json:"name"`
	Status           Status `json:"status"`
	RuntimeID        string `json:"runtimeId"`
	HermesHomePath   string `json:"hermesHomePath"`
	LastErrorCode    string `json:"lastErrorCode"`
	LastErrorMessage string `json:"lastErrorMessage"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type Runtime struct {
	AgentID          string `json:"agentId"`
	RuntimeID        string `json:"runtimeId"`
	Status           Status `json:"status"`
	LastErrorCode    string `json:"lastErrorCode"`
	LastErrorMessage string `json:"lastErrorMessage"`
	UpdatedAt        string `json:"updatedAt"`
}

type CreateParams struct {
	OwnerUserID string
	TemplateID  string
	Name        string
}

var transitions = map[Status]map[Status]struct{}{
	StatusCreating: {
		StatusProvisioning: {},
		StatusError:        {},
	},
	StatusProvisioning: {
		StatusStarting: {},
		StatusError:    {},
	},
	StatusStarting: {
		StatusRunning: {},
		StatusError:   {},
	},
	StatusRunning: {
		StatusStopped: {},
		StatusError:   {},
	},
	StatusStopped: {
		StatusStarting: {},
	},
	StatusError: {
		StatusProvisioning: {},
		StatusStarting:     {},
	},
}

func (s Status) CanTransitionTo(next Status) bool {
	nextStates, ok := transitions[s]
	if !ok {
		return false
	}
	_, ok = nextStates[next]
	return ok
}

func (s Status) CanRestartRuntime() bool {
	switch s {
	case StatusRunning, StatusStopped, StatusError:
		return true
	default:
		return false
	}
}

// CanDelete reports whether an agent in this status is eligible to be
// deleted. Only stable states are eligible; provisioning/starting are
// rejected to avoid races with RuntimeWorker. error is included to allow
// retries after a partially-completed deletion.
func (s Status) CanDelete() bool {
	switch s {
	case StatusCreating, StatusRunning, StatusStopped, StatusError:
		return true
	default:
		return false
	}
}
