package templates

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FileStore struct {
	dataDir string
}

type TemplatePaths struct {
	TemplatePath string
	SoulMDPath   string
	UserMDPath   string
	SkillsPath   string
}

func NewFileStore(dataDir string) *FileStore {
	return &FileStore{dataDir: dataDir}
}

func (s *FileStore) Paths(templateID string, version int) TemplatePaths {
	versionDir := filepath.Join(s.dataDir, "templates", templateID, "versions", strconv.Itoa(version))
	return TemplatePaths{
		TemplatePath: versionDir,
		SoulMDPath:   filepath.Join(versionDir, "SOUL.md"),
		UserMDPath:   filepath.Join(versionDir, "USER.md"),
		SkillsPath:   filepath.Join(versionDir, "skills"),
	}
}

func (s *FileStore) CreateTemplate(templateID string, version int) (TemplatePaths, error) {
	paths := s.Paths(templateID, version)
	if err := os.MkdirAll(paths.SkillsPath, 0o755); err != nil {
		return TemplatePaths{}, err
	}
	return paths, nil
}

func (s *FileStore) CopyTemplateVersion(source Template, targetID string, targetVersion int) (TemplatePaths, error) {
	targetPaths, err := s.CreateTemplate(targetID, targetVersion)
	if err != nil {
		return TemplatePaths{}, err
	}
	if err := copyFileIfExists(source.SoulMDPath, targetPaths.SoulMDPath); err != nil {
		return TemplatePaths{}, err
	}
	if err := copyFileIfExists(source.UserMDPath, targetPaths.UserMDPath); err != nil {
		return TemplatePaths{}, err
	}
	if err := copyDirIfExists(source.SkillsPath, targetPaths.SkillsPath); err != nil {
		return TemplatePaths{}, err
	}
	return targetPaths, nil
}

func (s *FileStore) WriteSoul(template Template, content string) error {
	return writeFile(template.SoulMDPath, content)
}

func (s *FileStore) ReadSoul(template Template) (string, error) {
	return readFile(template.SoulMDPath)
}

func (s *FileStore) WriteUser(template Template, content string) error {
	return writeFile(template.UserMDPath, content)
}

func (s *FileStore) ReadUser(template Template) (string, error) {
	return readFile(template.UserMDPath)
}

func (s *FileStore) WriteSkill(template Template, skillName, content string) (string, error) {
	skillPath := filepath.Join(template.SkillsPath, skillName, "SKILL.md")
	return checksumString(content), writeFile(skillPath, content)
}

func (s *FileStore) ImportSkillArchive(template Template, skillName string, content []byte) (string, error) {
	if !validSkillName(skillName) || len(content) == 0 {
		return "", ErrInvalidInput
	}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", ErrInvalidInput
	}

	type archiveEntry struct {
		path string
		file *zip.File
	}

	var root string
	wrappedRoot := true
	entries := make([]archiveEntry, 0, len(reader.File))
	for _, file := range reader.File {
		cleaned := filepath.Clean(file.Name)
		if cleaned == "." || strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
			return "", ErrInvalidInput
		}
		parts := strings.Split(cleaned, string(filepath.Separator))
		if len(parts) == 0 || parts[0] == "" {
			return "", ErrInvalidInput
		}
		if len(parts) < 2 {
			wrappedRoot = false
		}
		if root == "" {
			root = parts[0]
		} else if parts[0] != root {
			wrappedRoot = false
		}
		entries = append(entries, archiveEntry{path: cleaned, file: file})
	}
	if len(entries) == 0 {
		return "", ErrInvalidInput
	}
	if !wrappedRoot {
		root = ""
	}

	skillDir := filepath.Join(template.SkillsPath, skillName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return "", err
	}
	hasSkillMD := false
	for _, entry := range entries {
		relative := entry.path
		if root != "" {
			trimmed := strings.TrimPrefix(relative, root)
			trimmed = strings.TrimPrefix(trimmed, string(filepath.Separator))
			if trimmed == "" {
				continue
			}
			relative = trimmed
		}
		targetPath := filepath.Join(skillDir, relative)
		if !strings.HasPrefix(targetPath, skillDir) {
			return "", ErrInvalidInput
		}
		if entry.file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return "", err
			}
			continue
		}
		reader, err := entry.file.Open()
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return "", err
		}
		if relative == "SKILL.md" {
			hasSkillMD = true
		}
	}
	if !hasSkillMD {
		_ = os.RemoveAll(skillDir)
		return "", ErrInvalidInput
	}
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	skillMD, err := os.ReadFile(skillMDPath)
	if err != nil {
		_ = os.RemoveAll(skillDir)
		return "", err
	}
	if !hasRequiredSkillFrontmatter(string(skillMD)) {
		_ = os.RemoveAll(skillDir)
		return "", ErrInvalidInput
	}
	return checksumString(string(skillMD)), nil
}

func (s *FileStore) WriteSkillArchive(template Template, skillName string, files map[string][]byte) (string, error) {
	skillRoot := filepath.Join(template.SkillsPath, skillName)
	for relative, content := range files {
		if relative == "" || relative == "." {
			return "", ErrInvalidInput
		}
		targetPath := filepath.Join(skillRoot, relative)
		if !isPathWithin(skillRoot, targetPath) {
			return "", ErrInvalidInput
		}
		if err := writeFileBytes(targetPath, content); err != nil {
			return "", err
		}
	}
	skillMD, ok := files["SKILL.md"]
	if !ok || !hasRequiredSkillFrontmatter(string(skillMD)) {
		_ = os.RemoveAll(skillRoot)
		return "", ErrInvalidInput
	}
	return checksumBytes(files), nil
}

func (s *FileStore) ReadSkill(skill Skill) (string, error) {
	return readFile(skill.SkillPath)
}

func (s *FileStore) DeleteSkill(skill Skill) error {
	return os.RemoveAll(filepath.Dir(skill.SkillPath))
}

func (s *FileStore) MoveSkillToTrash(skill Skill) (string, error) {
	skillDir := filepath.Dir(skill.SkillPath)
	trashDir := filepath.Join(s.dataDir, ".trash", "skill-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if _, err := os.Stat(skillDir); errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	} else if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(trashDir), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(skillDir, trashDir); err != nil {
		return "", err
	}
	return trashDir, nil
}

func (s *FileStore) RestoreSkillFromTrash(skill Skill, trashDir string) error {
	if trashDir == "" {
		return nil
	}
	return os.Rename(trashDir, filepath.Dir(skill.SkillPath))
}

func (s *FileStore) RemoveTrash(path string) error {
	if path == "" {
		return nil
	}
	return os.RemoveAll(path)
}

func (s *FileStore) DeleteTemplate(template Template) error {
	if template.TemplatePath == "" {
		return nil
	}
	return os.RemoveAll(filepath.Dir(filepath.Dir(template.TemplatePath)))
}

func (s *FileStore) Checksum(template Template) (string, error) {
	hash := sha256.New()
	if _, err := os.Stat(template.TemplatePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return checksumString(""), nil
		}
		return "", err
	}

	var files []string
	if err := filepath.WalkDir(template.TemplatePath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, path := range files {
		relative, err := filepath.Rel(template.TemplatePath, path)
		if err != nil {
			return "", err
		}
		if _, err := hash.Write([]byte(relative)); err != nil {
			return "", err
		}
		if _, err := hash.Write([]byte{0}); err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(hash, file); err != nil {
			_ = file.Close()
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
		if _, err := hash.Write([]byte{0}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func checksumString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeFileBytes(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func copyFileIfExists(source, target string) error {
	input, err := os.Open(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func copyDirIfExists(source, target string) error {
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		return copyFileIfExists(path, targetPath)
	})
}

func hasRequiredSkillFrontmatter(content string) bool {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	foundName := false
	foundDescription := false
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			return foundName && foundDescription
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			foundName = true
		case "description":
			foundDescription = true
		}
	}
	return false
}

func checksumBytes(files map[string][]byte) string {
	hash := sha256.New()
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, _ = hash.Write([]byte(key))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(files[key])
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func isPathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
