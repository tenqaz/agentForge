package templates

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestServicePublishesOnlyCompleteTemplates(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	template, err := service.Create(ctx, "admin-1", "Support Agent", "answers customers")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := service.Publish(ctx, template.ID); !errors.Is(err, ErrInvalidTemplate) {
		t.Fatalf("Publish without content error = %v, want ErrInvalidTemplate", err)
	}

	if _, err := service.PutSoul(ctx, template.ID, "You are helpful."); err != nil {
		t.Fatalf("PutSoul returned error: %v", err)
	}
	if _, err := service.Publish(ctx, template.ID); !errors.Is(err, ErrInvalidTemplate) {
		t.Fatalf("Publish without USER.md error = %v, want ErrInvalidTemplate", err)
	}

	if _, err := service.PutUser(ctx, template.ID, "Use concise answers."); err != nil {
		t.Fatalf("PutUser returned error: %v", err)
	}
	if _, err := service.AddSkill(ctx, template.ID, "faq", "# FAQ\n"); err != nil {
		t.Fatalf("AddSkill returned error: %v", err)
	}

	published, err := service.Publish(ctx, template.ID)
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if published.Status != StatusPublished || published.PublishedAt == nil {
		t.Fatalf("published template = %#v", published)
	}
}

func TestServiceCreatesDraftWhenEditingPublishedTemplate(t *testing.T) {
	service, dataDir := newTestService(t)
	ctx := context.Background()
	published := createPublishableTemplate(t, service)

	next, err := service.PutSoul(ctx, published.ID, "Changed soul.")
	if err != nil {
		t.Fatalf("PutSoul published returned error: %v", err)
	}
	if next.ID == published.ID {
		t.Fatal("editing published template reused the published template id")
	}
	if next.Version != published.Version+1 || next.Status != StatusDraft {
		t.Fatalf("next draft = %#v, published = %#v", next, published)
	}

	originalSoul, err := os.ReadFile(filepath.Join(dataDir, "templates", published.ID, "versions", "1", "SOUL.md"))
	if err != nil {
		t.Fatalf("read original SOUL.md: %v", err)
	}
	if string(originalSoul) != "Original soul." {
		t.Fatalf("original SOUL.md = %q, want immutable original", originalSoul)
	}
	nextSoul, err := service.Soul(ctx, next.ID)
	if err != nil {
		t.Fatalf("Soul next returned error: %v", err)
	}
	if nextSoul != "Changed soul." {
		t.Fatalf("next soul = %q", nextSoul)
	}

	skills, err := service.ListSkills(ctx, next.ID)
	if err != nil {
		t.Fatalf("ListSkills next returned error: %v", err)
	}
	if len(skills) != 1 || skills[0].SkillName != "faq" {
		t.Fatalf("copied skills = %#v", skills)
	}
}

func TestServiceRejectsSkillDirectoryWithoutSkillMD(t *testing.T) {
	service, dataDir := newTestService(t)
	ctx := context.Background()
	template, err := service.Create(ctx, "admin-1", "Support Agent", "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := service.PutSoul(ctx, template.ID, "Original soul."); err != nil {
		t.Fatalf("PutSoul returned error: %v", err)
	}
	if _, err := service.PutUser(ctx, template.ID, "Original user."); err != nil {
		t.Fatalf("PutUser returned error: %v", err)
	}
	brokenSkillDir := filepath.Join(dataDir, "templates", template.ID, "versions", "1", "skills", "broken")
	if err := os.MkdirAll(brokenSkillDir, 0o755); err != nil {
		t.Fatalf("create broken skill dir: %v", err)
	}

	if _, err := service.Publish(ctx, template.ID); !errors.Is(err, ErrInvalidTemplate) {
		t.Fatalf("Publish with broken skill dir error = %v, want ErrInvalidTemplate", err)
	}
}

func TestServiceRejectsDuplicateSkillNameAndDeletesSkillDirectory(t *testing.T) {
	service, dataDir := newTestService(t)
	ctx := context.Background()
	template, err := service.Create(ctx, "admin-1", "Support Agent", "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	skill, err := service.AddSkill(ctx, template.ID, "faq", "# FAQ\n")
	if err != nil {
		t.Fatalf("AddSkill returned error: %v", err)
	}
	if _, err := service.AddSkill(ctx, template.ID, "faq", "# FAQ duplicate\n"); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate AddSkill error = %v, want ErrConflict", err)
	}

	if _, err := service.DeleteSkill(ctx, template.ID, skill.ID); err != nil {
		t.Fatalf("DeleteSkill returned error: %v", err)
	}
	if _, _, err := service.GetSkill(ctx, template.ID, skill.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSkill after delete error = %v, want ErrNotFound", err)
	}
	skillDir := filepath.Join(dataDir, "templates", template.ID, "versions", "1", "skills", "faq")
	if _, err := os.Stat(skillDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("skill dir stat error = %v, want not exist", err)
	}
}

func TestServiceRejectsInvalidSkillName(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	template, err := service.Create(ctx, "admin-1", "Support Agent", "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	for _, skillName := range []string{"../x", "a/b"} {
		if _, err := service.AddSkill(ctx, template.ID, skillName, "# Skill\n"); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("AddSkill(%q) error = %v, want ErrInvalidInput", skillName, err)
		}
	}
}

func TestRepositoryCreateSkillMapsUniqueConstraintToConflict(t *testing.T) {
	database := newTemplatesTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	template := Template{
		ID:              "template-1",
		Name:            "Support Agent",
		Status:          StatusDraft,
		Version:         1,
		TemplatePath:    "/tmp/template-1",
		ContentChecksum: checksumString(""),
		SoulMDPath:      "/tmp/template-1/SOUL.md",
		UserMDPath:      "/tmp/template-1/USER.md",
		SkillsPath:      "/tmp/template-1/skills",
		CreatedBy:       "admin-1",
	}
	if _, err := repository.CreateTemplate(ctx, template); err != nil {
		t.Fatalf("CreateTemplate returned error: %v", err)
	}
	if _, err := repository.CreateSkill(ctx, Skill{
		ID:         "skill-1",
		TemplateID: template.ID,
		SkillName:  "faq",
		SkillPath:  "/tmp/template-1/skills/faq/SKILL.md",
		Checksum:   checksumString("# FAQ\n"),
	}); err != nil {
		t.Fatalf("CreateSkill first returned error: %v", err)
	}
	if _, err := repository.CreateSkill(ctx, Skill{
		ID:         "skill-2",
		TemplateID: template.ID,
		SkillName:  "faq",
		SkillPath:  "/tmp/template-1/skills/faq/SKILL.md",
		Checksum:   checksumString("# FAQ\n"),
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateSkill duplicate error = %v, want ErrConflict", err)
	}
}

func TestServiceDeleteSkillKeepsDBRecordWhenFileDeletionFails(t *testing.T) {
	service, dataDir := newTestService(t)
	ctx := context.Background()
	template, err := service.Create(ctx, "admin-1", "Support Agent", "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	skill, err := service.AddSkill(ctx, template.ID, "faq", "# FAQ\n")
	if err != nil {
		t.Fatalf("AddSkill returned error: %v", err)
	}
	skillsDir := filepath.Join(dataDir, "templates", template.ID, "versions", "1", "skills")
	if err := os.Chmod(skillsDir, 0o555); err != nil {
		t.Fatalf("chmod skills dir read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(skillsDir, 0o755) })

	if _, err := service.DeleteSkill(ctx, template.ID, skill.ID); err == nil {
		t.Fatal("DeleteSkill returned nil error, want file deletion failure")
	}
	if _, _, err := service.GetSkill(ctx, template.ID, skill.ID); err != nil {
		t.Fatalf("GetSkill after failed delete error = %v, want record retained", err)
	}
}

func createPublishableTemplate(t *testing.T, service *Service) Template {
	t.Helper()
	ctx := context.Background()
	template, err := service.Create(ctx, "admin-1", "Support Agent", "answers customers")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := service.PutSoul(ctx, template.ID, "Original soul."); err != nil {
		t.Fatalf("PutSoul returned error: %v", err)
	}
	if _, err := service.PutUser(ctx, template.ID, "Original user."); err != nil {
		t.Fatalf("PutUser returned error: %v", err)
	}
	if _, err := service.AddSkill(ctx, template.ID, "faq", "# FAQ\n"); err != nil {
		t.Fatalf("AddSkill returned error: %v", err)
	}
	published, err := service.Publish(ctx, template.ID)
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	return published
}

func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	database := newTemplatesTestDB(t)
	dataDir := t.TempDir()
	return NewService(NewRepository(database), NewFileStore(dataDir)), dataDir
}

func newTemplatesTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", "file:templates-test-"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(`
		CREATE TABLE agent_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
			version INTEGER NOT NULL DEFAULT 1,
			template_path TEXT NOT NULL,
			content_checksum TEXT NOT NULL,
			soul_md_path TEXT NOT NULL,
			user_md_path TEXT NOT NULL,
			skills_path TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			published_at TEXT
		);
		CREATE TABLE template_skills (
			id TEXT PRIMARY KEY,
			template_id TEXT NOT NULL,
			skill_name TEXT NOT NULL,
			skill_path TEXT NOT NULL,
			checksum TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (template_id) REFERENCES agent_templates(id) ON DELETE CASCADE,
			UNIQUE (template_id, skill_name)
		);
	`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return database
}
