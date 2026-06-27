package templates

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// MaxSkillsPerTemplate 限制每个 template 最多可拥有的 skill 数量。
const MaxSkillsPerTemplate = 20

// AgentChecker defines the interface for checking if a template is in use by agents.
type AgentChecker interface {
	HasAgentsForTemplate(ctx context.Context, templateID string) (bool, error)
}

type Service struct {
	repository   *Repository
	store        *FileStore
	agentChecker AgentChecker
}

type DeleteSkillResult struct {
	Template Template
	Cloned   bool
}

func NewService(repository *Repository, store *FileStore, agentChecker ...AgentChecker) *Service {
	var ac AgentChecker
	if len(agentChecker) > 0 && agentChecker[0] != nil {
		ac = agentChecker[0]
	}
	return &Service{
		repository:   repository,
		store:        store,
		agentChecker: ac,
	}
}

func (s *Service) Create(ctx context.Context, createdBy, name, description string) (Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, fmt.Errorf("%w: template name cannot be empty", ErrInvalidInput)
	}
	id := uuid.NewString()
	version := 1
	paths, err := s.store.CreateTemplate(id, version)
	if err != nil {
		return Template{}, err
	}
	template := Template{
		ID:              id,
		Name:            name,
		Description:     description,
		Status:          StatusDraft,
		Version:         version,
		TemplatePath:    paths.TemplatePath,
		ContentChecksum: checksumString(""),
		SkillsPath:      paths.SkillsPath,
		CreatedBy:       createdBy,
	}
	created, err := s.repository.CreateTemplate(ctx, template)
	if err != nil {
		_ = s.store.DeleteTemplate(template)
		return Template{}, err
	}
	return created, nil
}

func (s *Service) CreateWithContents(ctx context.Context, params CreateTemplateParams) (Template, error) {
	template, err := s.Create(ctx, params.CreatedBy, params.Name, params.Description)
	if err != nil {
		return Template{}, err
	}
	trimmedSoul := strings.TrimSpace(params.SoulContent)
	if trimmedSoul == "" {
		_ = s.repository.DeleteTemplate(ctx, template.ID)
		_ = s.store.DeleteTemplate(template)
		return Template{}, fmt.Errorf("%w: soul content cannot be empty", ErrInvalidInput)
	}
	if _, err := s.repository.UpdateSoulContent(ctx, template.ID, params.SoulContent); err != nil {
		_ = s.repository.DeleteTemplate(ctx, template.ID)
		_ = s.store.DeleteTemplate(template)
		return Template{}, err
	}
	template, err = s.repository.UpdateUserContent(ctx, template.ID, params.UserContent)
	if err != nil {
		_ = s.repository.DeleteTemplate(ctx, template.ID)
		_ = s.store.DeleteTemplate(template)
		return Template{}, err
	}
	if len(params.SkillArchives) > MaxSkillsPerTemplate {
		_ = s.repository.DeleteTemplate(ctx, template.ID)
		_ = s.store.DeleteTemplate(template)
		return Template{}, fmt.Errorf("%w: cannot create template with more than %d skills", ErrInvalidInput, MaxSkillsPerTemplate)
	}
	for _, archive := range params.SkillArchives {
		skillName := strings.TrimSpace(strings.TrimSuffix(archive.Filename, filepath.Ext(archive.Filename)))
		if !validSkillName(skillName) {
			_ = s.repository.DeleteTemplate(ctx, template.ID)
			_ = s.store.DeleteTemplate(template)
			return Template{}, fmt.Errorf("%w: invalid skill name: %q", ErrInvalidInput, skillName)
		}
		if _, err := s.repository.FindSkillByName(ctx, template.ID, skillName); err == nil {
			_ = s.repository.DeleteTemplate(ctx, template.ID)
			_ = s.store.DeleteTemplate(template)
			return Template{}, ErrConflict
		} else if !errors.Is(err, ErrSkillNotFound) {
			_ = s.repository.DeleteTemplate(ctx, template.ID)
			_ = s.store.DeleteTemplate(template)
			return Template{}, err
		}
		checksum, err := s.store.ImportSkillArchive(template, skillName, archive.Content)
		if err != nil {
			_ = s.repository.DeleteTemplate(ctx, template.ID)
			_ = s.store.DeleteTemplate(template)
			// err 已经包含了 ErrInvalidInput 的包装，直接返回
			return Template{}, err
		}
		if _, err := s.repository.CreateSkill(ctx, Skill{
			ID:         uuid.NewString(),
			TemplateID: template.ID,
			SkillName:  skillName,
			SkillPath:  filepath.Join(template.SkillsPath, skillName, "SKILL.md"),
			Checksum:   checksum,
		}); err != nil {
			_ = s.repository.DeleteTemplate(ctx, template.ID)
			_ = s.store.DeleteTemplate(template)
			if errors.Is(err, ErrConflict) {
				return Template{}, ErrConflict
			}
			return Template{}, err
		}
	}
	return s.refreshChecksum(ctx, template)
}

func (s *Service) ListPublished(ctx context.Context) ([]Template, error) {
	return s.repository.ListTemplates(ctx, StatusPublished)
}

func (s *Service) ListAdmin(ctx context.Context) ([]Template, error) {
	return s.repository.ListTemplates(ctx, StatusDraft, StatusPublished)
}

func (s *Service) Get(ctx context.Context, id string) (Template, error) {
	return s.repository.GetTemplate(ctx, id)
}

func (s *Service) LoadPublishedTemplate(ctx context.Context, templateID string, version int) (Template, error) {
	template, err := s.repository.GetTemplate(ctx, templateID)
	if err != nil {
		return Template{}, err
	}
	if template.Status != StatusPublished || template.Version != version {
		return Template{}, ErrNotFound
	}
	return template, nil
}

func (s *Service) UpdateMetadata(ctx context.Context, id, name, description string) (Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, fmt.Errorf("%w: template name cannot be empty", ErrInvalidInput)
	}
	template, err := s.repository.GetTemplate(ctx, id)
	if err != nil {
		return Template{}, err
	}
	template, err = s.ensureDraft(ctx, template)
	if err != nil {
		return Template{}, err
	}
	return s.repository.UpdateTemplateMetadata(ctx, template.ID, name, description, template.ContentChecksum)
}

func (s *Service) Archive(ctx context.Context, id string) (Template, error) {
	return s.repository.ArchiveTemplate(ctx, id)
}

// Delete permanently deletes a template and its associated files.
// It first checks if the template is in use by any agents.
func (s *Service) Delete(ctx context.Context, id string) error {
	template, err := s.repository.GetTemplate(ctx, id)
	if err != nil {
		return err
	}

	// Check if template is in use by agents
	if s.agentChecker != nil {
		hasAgents, err := s.agentChecker.HasAgentsForTemplate(ctx, id)
		if err != nil {
			return fmt.Errorf("check template usage: %w", err)
		}
		if hasAgents {
			return ErrTemplateInUse
		}
	}

	// Delete template files first
	slog.DebugContext(ctx, "deleting template files",
		"template_id", id,
		"template_name", template.Name)
	if err := s.store.DeleteTemplate(template); err != nil {
		// Continue even if file deletion fails - we still want to clean up the database
		slog.WarnContext(ctx, "failed to delete template files, proceeding with database deletion",
			"template_id", id,
			"error", err)
	} else {
		slog.DebugContext(ctx, "template files deleted successfully",
			"template_id", id)
	}

	// Delete from database (skills are cascaded by foreign key)
	if err := s.repository.DeleteTemplate(ctx, id); err != nil {
		return fmt.Errorf("delete template from database: %w", err)
	}

	return nil
}

func (s *Service) PutSoul(ctx context.Context, id, content string) (Template, error) {
	template, err := s.editableTemplate(ctx, id)
	if err != nil {
		return Template{}, err
	}
	template, err = s.repository.UpdateSoulContent(ctx, template.ID, content)
	if err != nil {
		return Template{}, err
	}
	return s.refreshChecksum(ctx, template)
}

func (s *Service) Soul(ctx context.Context, id string) (string, error) {
	template, err := s.repository.GetTemplate(ctx, id)
	if err != nil {
		return "", err
	}
	return template.SoulContent, nil
}

func (s *Service) PutUser(ctx context.Context, id, content string) (Template, error) {
	template, err := s.editableTemplate(ctx, id)
	if err != nil {
		return Template{}, err
	}
	template, err = s.repository.UpdateUserContent(ctx, template.ID, content)
	if err != nil {
		return Template{}, err
	}
	return s.refreshChecksum(ctx, template)
}

func (s *Service) User(ctx context.Context, id string) (string, error) {
	template, err := s.repository.GetTemplate(ctx, id)
	if err != nil {
		return "", err
	}
	return template.UserContent, nil
}

func (s *Service) ListSkills(ctx context.Context, templateID string) ([]Skill, error) {
	if _, err := s.repository.GetTemplate(ctx, templateID); err != nil {
		return nil, err
	}
	return s.repository.ListSkills(ctx, templateID)
}

func (s *Service) AddSkill(ctx context.Context, templateID, skillName, skillMD string) (Skill, error) {
	skillName = strings.TrimSpace(skillName)
	if !validSkillName(skillName) {
		return Skill{}, fmt.Errorf("%w: invalid skill name: %q", ErrInvalidInput, skillName)
	}
	if strings.TrimSpace(skillMD) == "" {
		return Skill{}, fmt.Errorf("%w: skill content cannot be empty", ErrInvalidInput)
	}
	template, err := s.editableTemplate(ctx, templateID)
	if err != nil {
		return Skill{}, err
	}
	existing, err := s.repository.ListSkills(ctx, template.ID)
	if err != nil {
		return Skill{}, err
	}
	if len(existing) >= MaxSkillsPerTemplate {
		return Skill{}, fmt.Errorf("%w: template already has the maximum of %d skills", ErrInvalidInput, MaxSkillsPerTemplate)
	}
	if _, err := s.repository.FindSkillByName(ctx, template.ID, skillName); err == nil {
		return Skill{}, ErrConflict
	} else if !errors.Is(err, ErrSkillNotFound) {
		return Skill{}, err
	}
	checksum, err := s.store.WriteSkill(template, skillName, skillMD)
	if err != nil {
		return Skill{}, err
	}
	skillPath := filepath.Join(template.SkillsPath, skillName, "SKILL.md")
	skill, err := s.repository.CreateSkill(ctx, Skill{
		ID:         uuid.NewString(),
		TemplateID: template.ID,
		SkillName:  skillName,
		SkillPath:  skillPath,
		Checksum:   checksum,
	})
	if err != nil {
		_ = s.store.DeleteSkill(Skill{SkillPath: skillPath})
		if errors.Is(err, ErrConflict) {
			return Skill{}, ErrConflict
		}
		return Skill{}, err
	}
	if _, err := s.refreshChecksum(ctx, template); err != nil {
		return Skill{}, err
	}
	return skill, nil
}

func (s *Service) AddSkillArchive(ctx context.Context, templateID string, archive []byte) (Skill, error) {
	files, skillName, err := parseSkillArchive(archive)
	if err != nil {
		return Skill{}, err
	}
	template, err := s.editableTemplate(ctx, templateID)
	if err != nil {
		return Skill{}, err
	}
	existing, err := s.repository.ListSkills(ctx, template.ID)
	if err != nil {
		return Skill{}, err
	}
	if len(existing) >= MaxSkillsPerTemplate {
		return Skill{}, fmt.Errorf("%w: template already has the maximum of %d skills", ErrInvalidInput, MaxSkillsPerTemplate)
	}
	if _, err := s.repository.FindSkillByName(ctx, template.ID, skillName); err == nil {
		return Skill{}, ErrConflict
	} else if !errors.Is(err, ErrSkillNotFound) {
		return Skill{}, err
	}
	checksum, err := s.store.WriteSkillArchive(template, skillName, files)
	if err != nil {
		return Skill{}, err
	}
	skillPath := filepath.Join(template.SkillsPath, skillName, "SKILL.md")
	skill, err := s.repository.CreateSkill(ctx, Skill{
		ID:         uuid.NewString(),
		TemplateID: template.ID,
		SkillName:  skillName,
		SkillPath:  skillPath,
		Checksum:   checksum,
	})
	if err != nil {
		_ = s.store.DeleteSkill(Skill{SkillPath: skillPath})
		if errors.Is(err, ErrConflict) {
			return Skill{}, ErrConflict
		}
		return Skill{}, err
	}
	if _, err := s.refreshChecksum(ctx, template); err != nil {
		return Skill{}, err
	}
	return skill, nil
}

func (s *Service) GetSkill(ctx context.Context, templateID, skillID string) (Skill, string, error) {
	skill, err := s.repository.GetSkill(ctx, templateID, skillID)
	if errors.Is(err, ErrSkillNotFound) {
		return Skill{}, "", ErrNotFound
	}
	if err != nil {
		return Skill{}, "", err
	}
	content, err := s.store.ReadSkill(skill)
	if err != nil {
		return Skill{}, "", err
	}
	return skill, content, nil
}

func (s *Service) DeleteSkill(ctx context.Context, templateID, skillID string) (DeleteSkillResult, error) {
	sourceSkill, err := s.repository.GetSkill(ctx, templateID, skillID)
	if errors.Is(err, ErrSkillNotFound) {
		return DeleteSkillResult{}, ErrNotFound
	}
	if err != nil {
		return DeleteSkillResult{}, err
	}
	template, err := s.repository.GetTemplate(ctx, templateID)
	if err != nil {
		return DeleteSkillResult{}, err
	}
	originalTemplateID := template.ID
	template, err = s.ensureDraft(ctx, template)
	if err != nil {
		return DeleteSkillResult{}, err
	}
	skill := sourceSkill
	if template.ID != templateID {
		skill, err = s.repository.FindSkillByName(ctx, template.ID, sourceSkill.SkillName)
		if err != nil {
			return DeleteSkillResult{}, err
		}
	}
	trashDir, err := s.store.MoveSkillToTrash(skill)
	if err != nil {
		return DeleteSkillResult{}, err
	}
	if err := s.repository.DeleteSkill(ctx, template.ID, skill.ID); err != nil {
		_ = s.store.RestoreSkillFromTrash(skill, trashDir)
		if errors.Is(err, ErrNotFound) {
			return DeleteSkillResult{}, ErrNotFound
		}
		return DeleteSkillResult{}, err
	}
	if err := s.store.RemoveTrash(trashDir); err != nil {
		return DeleteSkillResult{}, err
	}
	template, err = s.refreshChecksum(ctx, template)
	if err != nil {
		return DeleteSkillResult{}, err
	}
	return DeleteSkillResult{Template: template, Cloned: template.ID != originalTemplateID}, nil
}

func (s *Service) Publish(ctx context.Context, templateID string) (Template, error) {
	template, err := s.repository.GetTemplate(ctx, templateID)
	if err != nil {
		return Template{}, err
	}
	if err := s.validatePublishable(ctx, template); err != nil {
		return Template{}, err
	}
	checksum, err := s.store.Checksum(template)
	if err != nil {
		return Template{}, err
	}
	return s.repository.PublishTemplate(ctx, template.ID, checksum)
}

func (s *Service) Unpublish(ctx context.Context, templateID string) (Template, error) {
	template, err := s.repository.GetTemplate(ctx, templateID)
	if err != nil {
		return Template{}, err
	}
	if template.Status != StatusPublished {
		return Template{}, fmt.Errorf("%w: template is not published (status: %q)", ErrInvalidInput, template.Status)
	}
	next, err := s.ensureDraft(ctx, template)
	if err != nil {
		return Template{}, err
	}
	if _, err := s.repository.ArchiveTemplate(ctx, template.ID); err != nil {
		_ = s.repository.DeleteTemplate(ctx, next.ID)
		_ = s.store.DeleteTemplate(next)
		return Template{}, err
	}
	return next, nil
}

func (s *Service) editableTemplate(ctx context.Context, id string) (Template, error) {
	template, err := s.repository.GetTemplate(ctx, id)
	if err != nil {
		return Template{}, err
	}
	return s.ensureDraft(ctx, template)
}

func (s *Service) ensureDraft(ctx context.Context, template Template) (Template, error) {
	if template.Status == StatusDraft {
		return template, nil
	}
	nextID := uuid.NewString()
	nextVersion := template.Version + 1
	paths, err := s.store.CopyTemplateVersion(template, nextID, nextVersion)
	if err != nil {
		return Template{}, err
	}
	next := Template{
		ID:              nextID,
		Name:            template.Name,
		Description:     template.Description,
		Status:          StatusDraft,
		Version:         nextVersion,
		TemplatePath:    paths.TemplatePath,
		ContentChecksum: template.ContentChecksum,
		SoulContent:     template.SoulContent,
		UserContent:     template.UserContent,
		SkillsPath:      paths.SkillsPath,
		CreatedBy:       template.CreatedBy,
	}
	next, err = s.repository.CreateTemplate(ctx, next)
	if err != nil {
		_ = s.store.DeleteTemplate(next)
		return Template{}, err
	}
	sourceSkills, err := s.repository.ListSkills(ctx, template.ID)
	if err != nil {
		_ = s.repository.DeleteTemplate(ctx, next.ID)
		_ = s.store.DeleteTemplate(next)
		return Template{}, err
	}
	for _, sourceSkill := range sourceSkills {
		content, err := os.ReadFile(filepath.Join(template.SkillsPath, sourceSkill.SkillName, "SKILL.md"))
		if err != nil {
			_ = s.repository.DeleteTemplate(ctx, next.ID)
			_ = s.store.DeleteTemplate(next)
			return Template{}, err
		}
		if _, err := s.repository.CreateSkill(ctx, Skill{
			ID:         uuid.NewString(),
			TemplateID: next.ID,
			SkillName:  sourceSkill.SkillName,
			SkillPath:  filepath.Join(paths.SkillsPath, sourceSkill.SkillName, "SKILL.md"),
			Checksum:   checksumString(string(content)),
		}); err != nil {
			_ = s.repository.DeleteTemplate(ctx, next.ID)
			_ = s.store.DeleteTemplate(next)
			return Template{}, err
		}
	}
	return s.refreshChecksum(ctx, next)
}

func (s *Service) refreshChecksum(ctx context.Context, template Template) (Template, error) {
	checksum, err := s.store.Checksum(template)
	if err != nil {
		return Template{}, err
	}
	return s.repository.UpdateTemplateChecksum(ctx, template.ID, checksum)
}

func (s *Service) validatePublishable(ctx context.Context, template Template) error {
	if strings.TrimSpace(template.SoulContent) == "" {
		return fmt.Errorf("%w: SOUL.md is empty", ErrInvalidTemplate)
	}
	if strings.TrimSpace(template.UserContent) == "" {
		return fmt.Errorf("%w: USER.md is empty", ErrInvalidTemplate)
	}
	skillDirs, err := os.ReadDir(template.SkillsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, skillDir := range skillDirs {
		if !skillDir.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(template.SkillsPath, skillDir.Name(), "SKILL.md")); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidTemplate, err)
		}
	}
	skills, err := s.repository.ListSkills(ctx, template.ID)
	if err != nil {
		return err
	}
	for _, skill := range skills {
		if _, err := os.Stat(skill.SkillPath); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidTemplate, err)
		}
	}
	return nil
}

func validSkillName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	return filepath.Clean(name) == name
}

func parseSkillArchive(archive []byte) (map[string][]byte, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	files := map[string][]byte{}
	var skillName string
	hasSkillMD := false
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		cleanName := filepath.ToSlash(filepath.Clean(file.Name))
		if cleanName == "." || strings.HasPrefix(cleanName, "../") || strings.Contains(cleanName, "/../") {
			return nil, "", fmt.Errorf("%w: invalid path in archive: %q", ErrInvalidInput, file.Name)
		}
		parts := strings.Split(cleanName, "/")
		if len(parts) < 2 {
			return nil, "", fmt.Errorf("%w: invalid path structure, expected skill directory: %q", ErrInvalidInput, file.Name)
		}
		if skillName == "" {
			skillName = parts[0]
			if !validSkillName(skillName) {
				return nil, "", fmt.Errorf("%w: invalid skill name: %q", ErrInvalidInput, skillName)
			}
		} else if parts[0] != skillName {
			return nil, "", fmt.Errorf("%w: multiple root directories in archive: %q and %q", ErrInvalidInput, skillName, parts[0])
		}
		relative := strings.Join(parts[1:], "/")
		if relative == "" || relative == "." || strings.HasPrefix(relative, "../") || strings.Contains(relative, "/../") {
			return nil, "", fmt.Errorf("%w: invalid relative path: %q", ErrInvalidInput, relative)
		}
		if relative == "SKILL.md" {
			hasSkillMD = true
		}
		content, err := readZipFile(file)
		if err != nil {
			return nil, "", err
		}
		files[relative] = content
	}
	if skillName == "" || !hasSkillMD {
		return nil, "", fmt.Errorf("%w: missing skill name or SKILL.md", ErrInvalidInput)
	}
	return files, skillName, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return content, nil
}
