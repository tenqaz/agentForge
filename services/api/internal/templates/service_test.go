package templates

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	service, _ := newTestService(t)
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

	originalSoul, err := service.Soul(ctx, published.ID)
	if err != nil {
		t.Fatalf("Soul published returned error: %v", err)
	}
	if originalSoul != "Original soul." {
		t.Fatalf("original soul = %q, want immutable original", originalSoul)
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

func TestServiceCreateWithContentsWritesSoulUserAndImportsSkillArchives(t *testing.T) {
	service, dataDir := newTestService(t)
	ctx := context.Background()

	template, err := service.CreateWithContents(ctx, CreateTemplateParams{
		CreatedBy:   "admin-1",
		Name:        "Support Agent",
		Description: "answers customers",
		SoulContent: "# Soul\nCalm and direct.",
		UserContent: "# User\nKeep answers short.",
		SkillArchives: []SkillArchive{
			{
				Filename: "faq.zip",
				Content:  createSkillArchive(t, map[string]string{"SKILL.md": "---\nname: FAQ\ndescription: Frequently asked questions\n---\n# FAQ\n"}),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWithContents returned error: %v", err)
	}

	soul, err := service.Soul(ctx, template.ID)
	if err != nil {
		t.Fatalf("Soul returned error: %v", err)
	}
	if soul != "# Soul\nCalm and direct." {
		t.Fatalf("soul = %q", soul)
	}
	userContent, err := service.User(ctx, template.ID)
	if err != nil {
		t.Fatalf("User returned error: %v", err)
	}
	if userContent != "# User\nKeep answers short." {
		t.Fatalf("userContent = %q", userContent)
	}
	skills, err := service.ListSkills(ctx, template.ID)
	if err != nil {
		t.Fatalf("ListSkills returned error: %v", err)
	}
	if len(skills) != 1 || skills[0].SkillName != "faq" {
		t.Fatalf("skills = %#v", skills)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "templates", template.ID, "versions", "1", "skills", "faq", "SKILL.md")); err != nil {
		t.Fatalf("skill file stat error: %v", err)
	}
}

func TestServiceCreateWithContentsRollsBackOnInvalidSkillArchive(t *testing.T) {
	service, dataDir := newTestService(t)
	ctx := context.Background()

	_, err := service.CreateWithContents(ctx, CreateTemplateParams{
		CreatedBy:   "admin-1",
		Name:        "Broken Agent",
		Description: "broken",
		SoulContent: "# Soul\nBroken.",
		UserContent: "# User\nBroken.",
		SkillArchives: []SkillArchive{
			{
				Filename: "broken.zip",
				Content:  createSkillArchive(t, map[string]string{"notes.md": "missing skill"}),
			},
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateWithContents error = %v, want ErrInvalidInput", err)
	}

	templatesDir := filepath.Join(dataDir, "templates")
	entries, readErr := os.ReadDir(templatesDir)
	if errors.Is(readErr, os.ErrNotExist) {
		return
	}
	if readErr != nil {
		t.Fatalf("ReadDir returned error: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("templates dir entries = %d, want 0", len(entries))
	}
}

func TestServiceAddSkillRejectsBeyondLimit(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	template, err := service.Create(ctx, "admin-1", "Limit Agent", "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := service.PutSoul(ctx, template.ID, "Soul"); err != nil {
		t.Fatalf("PutSoul returned error: %v", err)
	}

	// 填满 10 个 skill
	for i := range MaxSkillsPerTemplate {
		name := "skill-" + string(rune('a'+i))
		if _, err := service.AddSkill(ctx, template.ID, name, "# "+name+"\n"); err != nil {
			t.Fatalf("AddSkill #%d returned error: %v", i, err)
		}
	}

	// 第 11 个应被拒绝
	_, err = service.AddSkill(ctx, template.ID, "overflow", "# overflow\n")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("AddSkill beyond limit error = %v, want ErrInvalidInput", err)
	}

	// 列表仍为 10 个，超限的没被写入
	skills, err := service.ListSkills(ctx, template.ID)
	if err != nil {
		t.Fatalf("ListSkills returned error: %v", err)
	}
	if len(skills) != MaxSkillsPerTemplate {
		t.Fatalf("skills count = %d, want %d", len(skills), MaxSkillsPerTemplate)
	}
}

func TestServiceAddSkillArchiveRejectsBeyondLimit(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	template, err := service.Create(ctx, "admin-1", "Limit Agent", "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := service.PutSoul(ctx, template.ID, "Soul"); err != nil {
		t.Fatalf("PutSoul returned error: %v", err)
	}
	for i := range MaxSkillsPerTemplate {
		name := "skill-" + string(rune('a'+i))
		if _, err := service.AddSkill(ctx, template.ID, name, "# "+name+"\n"); err != nil {
			t.Fatalf("AddSkill #%d returned error: %v", i, err)
		}
	}

	archive := createSkillArchive(t, map[string]string{
		"SKILL.md": "---\nname: overflow\ndescription: Eleventh skill\n---\n# Overflow\n",
	})
	_, err = service.AddSkillArchive(ctx, template.ID, archive)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("AddSkillArchive beyond limit error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCreateWithContentsRejectsTooManySkillArchives(t *testing.T) {
	service, dataDir := newTestService(t)
	ctx := context.Background()

	archives := make([]SkillArchive, 0, MaxSkillsPerTemplate+1)
	for i := range MaxSkillsPerTemplate + 1 {
		name := "skill" + string(rune('a'+i))
		archives = append(archives, SkillArchive{
			Filename: name + ".zip",
			Content: createSkillArchive(t, map[string]string{
				"SKILL.md": "---\nname: " + name + "\ndescription: test\n---\n# " + name + "\n",
			}),
		})
	}

	_, err := service.CreateWithContents(ctx, CreateTemplateParams{
		CreatedBy:     "admin-1",
		Name:          "TooMany",
		Description:   "too many",
		SoulContent:   "# Soul\n",
		UserContent:   "# User\n",
		SkillArchives: archives,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateWithContents error = %v, want ErrInvalidInput", err)
	}

	// 回滚后模板目录应为空
	templatesDir := filepath.Join(dataDir, "templates")
	entries, readErr := os.ReadDir(templatesDir)
	if errors.Is(readErr, os.ErrNotExist) {
		return
	}
	if readErr != nil {
		t.Fatalf("ReadDir returned error: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("templates dir entries = %d, want 0 after rollback", len(entries))
	}
}

// TestServiceCreateWithContentsHandlesSkillArchiveWithDirectoryEntries 复现真实
// zip 工具（zip CLI、Finder、Python shutil.make_archive）生成的归档：它们会写入
// 显式目录条目（如 "deep-analysis/"）。带目录条目的归档会让 ImportSkillArchive
// 的"包裹根目录"检测失效，导致 skill 名称被重复一层（skills/<name>/<name>/SKILL.md），
// 进而让发布校验找不到 skills/<name>/SKILL.md。
func TestServiceCreateWithContentsHandlesSkillArchiveWithDirectoryEntries(t *testing.T) {
	service, dataDir := newTestService(t)
	ctx := context.Background()

	archive := createSkillArchiveWithDirs(t, "deep-analysis", map[string]string{
		"SKILL.md":             "---\nname: deep-analysis\ndescription: Deep analysis\n---\n# Deep Analysis\n",
		"assets/avatars/a.svg": "<svg/>",
	})

	template, err := service.CreateWithContents(ctx, CreateTemplateParams{
		CreatedBy:   "admin-1",
		Name:        "Analyst Agent",
		Description: "deep analysis skill",
		SoulContent: "# Soul\nAnalytical.",
		UserContent: "# User\nBe thorough.",
		SkillArchives: []SkillArchive{
			{Filename: "deep-analysis.zip", Content: archive},
		},
	})
	if err != nil {
		t.Fatalf("CreateWithContents returned error: %v", err)
	}

	skillMD := filepath.Join(dataDir, "templates", template.ID, "versions", "1", "skills", "deep-analysis", "SKILL.md")
	if _, err := os.Stat(skillMD); err != nil {
		t.Fatalf("expected SKILL.md at %s, got error: %v", skillMD, err)
	}
	// 不应出现重复的目录层
	duplicated := filepath.Join(dataDir, "templates", template.ID, "versions", "1", "skills", "deep-analysis", "deep-analysis", "SKILL.md")
	if _, err := os.Stat(duplicated); err == nil {
		t.Fatalf("duplicated skill directory should not exist: %s", duplicated)
	}
	// 发布校验会 stat skills/<name>/SKILL.md，必须通过
	if _, err := service.Publish(ctx, template.ID); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
}

func TestServiceAddSkillArchiveWritesWholeSkillDirectory(t *testing.T) {
	service, dataDir := newTestService(t)
	ctx := context.Background()
	template, err := service.Create(ctx, "admin-1", "Support Agent", "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	skill, err := service.AddSkillArchive(ctx, template.ID, mustZipSkill(t, map[string]string{
		"handoff/SKILL.md":         "---\nname: handoff\ndescription: Escalation path\n---\n# SKILL\nEscalate to humans when needed.\n",
		"handoff/references/a.md":  "reference",
		"handoff/scripts/run.sh":   "#!/bin/sh\nexit 0\n",
	}))
	if err != nil {
		t.Fatalf("AddSkillArchive returned error: %v", err)
	}
	if skill.SkillName != "handoff" {
		t.Fatalf("skill name = %q, want handoff", skill.SkillName)
	}

	skillDir := filepath.Join(dataDir, "templates", template.ID, "versions", "1", "skills", "handoff")
	for _, relative := range []string{"SKILL.md", "references/a.md", "scripts/run.sh"} {
		if _, err := os.Stat(filepath.Join(skillDir, relative)); err != nil {
			t.Fatalf("expected skill file %s: %v", relative, err)
		}
	}
}

func TestServiceAddSkillArchiveRejectsInvalidArchives(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	template, err := service.Create(ctx, "admin-1", "Support Agent", "")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	tests := []struct {
		name    string
		archive []byte
	}{
		{
			name: "multiple top level directories",
			archive: mustZipSkill(t, map[string]string{
				"faq/SKILL.md": "# FAQ\n",
				"ops/SKILL.md": "# OPS\n",
			}),
		},
		{
			name: "missing skill md",
			archive: mustZipSkill(t, map[string]string{
				"faq/readme.md": "missing skill",
			}),
		},
		{
			name: "invalid top level directory name",
			archive: mustZipSkill(t, map[string]string{
				"../bad/SKILL.md": "# bad\n",
			}),
		},
		{
			name: "path traversal",
			archive: mustZipSkill(t, map[string]string{
				"faq/../oops.txt": "escape",
				"faq/SKILL.md":    "# FAQ\n",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.AddSkillArchive(ctx, template.ID, tt.archive); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("AddSkillArchive error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestServiceAddSkillArchiveCreatesDraftWhenEditingPublishedTemplate(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	published := createPublishableTemplate(t, service)

	skill, err := service.AddSkillArchive(ctx, published.ID, mustZipSkill(t, map[string]string{
		"handoff/SKILL.md": "---\nname: handoff\ndescription: Escalation path\n---\n# SKILL\nEscalate.\n",
	}))
	if err != nil {
		t.Fatalf("AddSkillArchive returned error: %v", err)
	}
	if skill.TemplateID == published.ID {
		t.Fatalf("published template was edited in place: %#v", skill)
	}

	skills, err := service.ListSkills(ctx, skill.TemplateID)
	if err != nil {
		t.Fatalf("ListSkills returned error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("skills after clone = %d, want 2", len(skills))
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
			soul_content TEXT NOT NULL DEFAULT '',
			user_content TEXT NOT NULL DEFAULT '',
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

func createSkillArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("Write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close zip writer: %v", err)
	}
	return buffer.Bytes()
}

// createSkillArchiveWithDirs 构造一个带显式目录条目的归档，模拟真实 zip 工具的输出。
// root 是包裹所有条目的根目录名（如 "deep-analysis"），files 是相对 root 的文件路径映射。
func createSkillArchiveWithDirs(t *testing.T, root string, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	dirs := map[string]bool{}
	for rel := range files {
		full := root + "/" + rel
		parts := strings.Split(full, "/")
		for i := 1; i < len(parts); i++ {
			dirs[strings.Join(parts[:i], "/")+"/"] = true
		}
	}
	for dir := range dirs {
		header := &zip.FileHeader{Name: dir, Method: zip.Store}
		header.SetMode(os.ModeDir | 0o755)
		if _, err := writer.CreateHeader(header); err != nil {
			t.Fatalf("Create dir entry %s: %v", dir, err)
		}
	}
	for rel, content := range files {
		entry, err := writer.Create(root + "/" + rel)
		if err != nil {
			t.Fatalf("Create zip entry %s: %v", root+"/"+rel, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("Write zip entry %s: %v", root+"/"+rel, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close zip writer: %v", err)
	}
	return buffer.Bytes()
}

func mustZipSkill(t *testing.T, files map[string]string) []byte {
	return createSkillArchive(t, files)
}
