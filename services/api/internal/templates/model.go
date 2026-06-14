package templates

import "errors"

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

var (
	ErrNotFound        = errors.New("template not found")
	ErrSkillNotFound   = errors.New("template skill not found")
	ErrConflict        = errors.New("template conflict")
	ErrInvalidInput    = errors.New("invalid template input")
	ErrInvalidTemplate = errors.New("invalid template")
)

type Template struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Status          Status  `json:"status"`
	Version         int     `json:"version"`
	TemplatePath    string  `json:"templatePath"`
	ContentChecksum string  `json:"contentChecksum"`
	SoulMDPath      string  `json:"soulMDPath"`
	UserMDPath      string  `json:"userMDPath"`
	SkillsPath      string  `json:"skillsPath"`
	CreatedBy       string  `json:"createdBy"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	PublishedAt     *string `json:"publishedAt,omitempty"`
}

type Skill struct {
	ID         string `json:"id"`
	TemplateID string `json:"templateId"`
	SkillName  string `json:"skillName"`
	SkillPath  string `json:"skillPath"`
	Checksum   string `json:"checksum"`
	CreatedAt  string `json:"createdAt"`
}
