package templates

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	repository *Repository
	store      *FileStore
}

type DeleteSkillResult struct {
	Template Template
	Cloned   bool
}

func NewService(repository *Repository, store *FileStore) *Service {
	return &Service{repository: repository, store: store}
}

func (s *Service) Create(ctx context.Context, createdBy, name, description string) (Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, ErrInvalidInput
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
		SoulMDPath:      paths.SoulMDPath,
		UserMDPath:      paths.UserMDPath,
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
	if strings.TrimSpace(params.SoulContent) == "" {
		_ = s.repository.DeleteTemplate(ctx, template.ID)
		_ = s.store.DeleteTemplate(template)
		return Template{}, ErrInvalidInput
	}
	if err := s.store.WriteSoul(template, params.SoulContent); err != nil {
		_ = s.repository.DeleteTemplate(ctx, template.ID)
		_ = s.store.DeleteTemplate(template)
		return Template{}, err
	}
	if err := s.store.WriteUser(template, params.UserContent); err != nil {
		_ = s.repository.DeleteTemplate(ctx, template.ID)
		_ = s.store.DeleteTemplate(template)
		return Template{}, err
	}
	for _, archive := range params.SkillArchives {
		skillName := strings.TrimSpace(strings.TrimSuffix(archive.Filename, filepath.Ext(archive.Filename)))
		if !validSkillName(skillName) {
			_ = s.repository.DeleteTemplate(ctx, template.ID)
			_ = s.store.DeleteTemplate(template)
			return Template{}, ErrInvalidInput
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
			if errors.Is(err, ErrInvalidInput) {
				return Template{}, ErrInvalidInput
			}
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
		return Template{}, ErrInvalidInput
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

func (s *Service) PutSoul(ctx context.Context, id, content string) (Template, error) {
	template, err := s.editableTemplate(ctx, id)
	if err != nil {
		return Template{}, err
	}
	if err := s.store.WriteSoul(template, content); err != nil {
		return Template{}, err
	}
	return s.refreshChecksum(ctx, template)
}

func (s *Service) Soul(ctx context.Context, id string) (string, error) {
	template, err := s.repository.GetTemplate(ctx, id)
	if err != nil {
		return "", err
	}
	return s.store.ReadSoul(template)
}

func (s *Service) PutUser(ctx context.Context, id, content string) (Template, error) {
	template, err := s.editableTemplate(ctx, id)
	if err != nil {
		return Template{}, err
	}
	if err := s.store.WriteUser(template, content); err != nil {
		return Template{}, err
	}
	return s.refreshChecksum(ctx, template)
}

func (s *Service) User(ctx context.Context, id string) (string, error) {
	template, err := s.repository.GetTemplate(ctx, id)
	if err != nil {
		return "", err
	}
	return s.store.ReadUser(template)
}

func (s *Service) ListSkills(ctx context.Context, templateID string) ([]Skill, error) {
	if _, err := s.repository.GetTemplate(ctx, templateID); err != nil {
		return nil, err
	}
	return s.repository.ListSkills(ctx, templateID)
}

func (s *Service) AddSkill(ctx context.Context, templateID, skillName, skillMD string) (Skill, error) {
	skillName = strings.TrimSpace(skillName)
	if !validSkillName(skillName) || strings.TrimSpace(skillMD) == "" {
		return Skill{}, ErrInvalidInput
	}
	template, err := s.editableTemplate(ctx, templateID)
	if err != nil {
		return Skill{}, err
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
		return Template{}, ErrInvalidInput
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
		SoulMDPath:      paths.SoulMDPath,
		UserMDPath:      paths.UserMDPath,
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
		content, err := os.ReadFile(filepath.Join(paths.SkillsPath, sourceSkill.SkillName, "SKILL.md"))
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
	soul, err := s.store.ReadSoul(template)
	if err != nil {
		return ErrInvalidTemplate
	}
	if strings.TrimSpace(soul) == "" {
		return ErrInvalidTemplate
	}
	if _, err := os.Stat(template.UserMDPath); err != nil {
		return ErrInvalidTemplate
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
			return ErrInvalidTemplate
		}
	}
	skills, err := s.repository.ListSkills(ctx, template.ID)
	if err != nil {
		return err
	}
	for _, skill := range skills {
		if _, err := os.Stat(skill.SkillPath); err != nil {
			return ErrInvalidTemplate
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
		return nil, "", ErrInvalidInput
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
			return nil, "", ErrInvalidInput
		}
		parts := strings.Split(cleanName, "/")
		if len(parts) < 2 {
			return nil, "", ErrInvalidInput
		}
		if skillName == "" {
			skillName = parts[0]
			if !validSkillName(skillName) {
				return nil, "", ErrInvalidInput
			}
		} else if parts[0] != skillName {
			return nil, "", ErrInvalidInput
		}
		relative := strings.Join(parts[1:], "/")
		if relative == "" || relative == "." || strings.HasPrefix(relative, "../") || strings.Contains(relative, "/../") {
			return nil, "", ErrInvalidInput
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
		return nil, "", ErrInvalidInput
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
