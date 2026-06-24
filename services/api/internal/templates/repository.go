package templates

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type Repository struct {
	database *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{database: database}
}

func (r *Repository) CreateTemplate(ctx context.Context, template Template) (Template, error) {
	_, err := r.database.ExecContext(ctx, `
		INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_md_path, user_md_path, soul_content, user_content, skills_path, created_by
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`, template.ID, template.Name, template.Description, template.Status, template.Version, template.TemplatePath,
		template.ContentChecksum, template.SoulMDPath, template.UserMDPath, template.SoulContent, template.UserContent,
		template.SkillsPath, template.CreatedBy)
	if err != nil {
		return Template{}, err
	}
	return r.GetTemplate(ctx, template.ID)
}

func (r *Repository) DeleteTemplate(ctx context.Context, id string) error {
	result, err := r.database.ExecContext(ctx, `
		DELETE FROM agent_templates
		WHERE id = ?;
	`, id)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (r *Repository) GetTemplate(ctx context.Context, id string) (Template, error) {
	var template Template
	var publishedAt sql.NullString
	err := r.database.QueryRowContext(ctx, `
		SELECT id, name, description, status, version, template_path, content_checksum,
		       soul_md_path, user_md_path, soul_content, user_content, skills_path,
		       created_by, created_at, updated_at, published_at
		FROM agent_templates
		WHERE id = ?;
	`, id).Scan(
		&template.ID, &template.Name, &template.Description, &template.Status, &template.Version,
		&template.TemplatePath, &template.ContentChecksum, &template.SoulMDPath, &template.UserMDPath,
		&template.SoulContent, &template.UserContent, &template.SkillsPath, &template.CreatedBy,
		&template.CreatedAt, &template.UpdatedAt, &publishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Template{}, ErrNotFound
	}
	if err != nil {
		return Template{}, err
	}
	if publishedAt.Valid {
		template.PublishedAt = &publishedAt.String
	}
	return template, nil
}

func (r *Repository) ListTemplates(ctx context.Context, statuses ...Status) ([]Template, error) {
	query := `
		SELECT id, name, description, status, version, template_path, content_checksum,
		       soul_md_path, user_md_path, soul_content, user_content, skills_path,
		       created_by, created_at, updated_at, published_at
		FROM agent_templates`
	var args []any
	if len(statuses) > 0 {
		placeholders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, status)
		}
		query += " WHERE status IN (" + strings.Join(placeholders, ", ") + ")"
	}
	query += " ORDER BY updated_at DESC, id ASC;"

	rows, err := r.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []Template
	for rows.Next() {
		var template Template
		var publishedAt sql.NullString
		if err := rows.Scan(
			&template.ID, &template.Name, &template.Description, &template.Status, &template.Version,
			&template.TemplatePath, &template.ContentChecksum, &template.SoulMDPath, &template.UserMDPath,
			&template.SoulContent, &template.UserContent, &template.SkillsPath, &template.CreatedBy,
			&template.CreatedAt, &template.UpdatedAt, &publishedAt,
		); err != nil {
			return nil, err
		}
		if publishedAt.Valid {
			template.PublishedAt = &publishedAt.String
		}
		templates = append(templates, template)
	}
	return templates, rows.Err()
}

func (r *Repository) UpdateTemplateMetadata(ctx context.Context, id, name, description, checksum string) (Template, error) {
	result, err := r.database.ExecContext(ctx, `
		UPDATE agent_templates
		SET name = ?, description = ?, content_checksum = ?, updated_at = datetime('now')
		WHERE id = ?;
	`, name, description, checksum, id)
	if err != nil {
		return Template{}, err
	}
	if err := requireAffected(result); err != nil {
		return Template{}, err
	}
	return r.GetTemplate(ctx, id)
}

func (r *Repository) UpdateTemplateChecksum(ctx context.Context, id, checksum string) (Template, error) {
	result, err := r.database.ExecContext(ctx, `
		UPDATE agent_templates
		SET content_checksum = ?, updated_at = datetime('now')
		WHERE id = ?;
	`, checksum, id)
	if err != nil {
		return Template{}, err
	}
	if err := requireAffected(result); err != nil {
		return Template{}, err
	}
	return r.GetTemplate(ctx, id)
}

func (r *Repository) UpdateSoulContent(ctx context.Context, id, content string) (Template, error) {
	result, err := r.database.ExecContext(ctx, `
		UPDATE agent_templates
		SET soul_content = ?, updated_at = datetime('now')
		WHERE id = ?;
	`, content, id)
	if err != nil {
		return Template{}, err
	}
	if err := requireAffected(result); err != nil {
		return Template{}, err
	}
	return r.GetTemplate(ctx, id)
}

func (r *Repository) UpdateUserContent(ctx context.Context, id, content string) (Template, error) {
	result, err := r.database.ExecContext(ctx, `
		UPDATE agent_templates
		SET user_content = ?, updated_at = datetime('now')
		WHERE id = ?;
	`, content, id)
	if err != nil {
		return Template{}, err
	}
	if err := requireAffected(result); err != nil {
		return Template{}, err
	}
	return r.GetTemplate(ctx, id)
}

func (r *Repository) PublishTemplate(ctx context.Context, id, checksum string) (Template, error) {
	result, err := r.database.ExecContext(ctx, `
		UPDATE agent_templates
		SET status = 'published', content_checksum = ?, published_at = datetime('now'), updated_at = datetime('now')
		WHERE id = ?;
	`, checksum, id)
	if err != nil {
		return Template{}, err
	}
	if err := requireAffected(result); err != nil {
		return Template{}, err
	}
	return r.GetTemplate(ctx, id)
}

func (r *Repository) ArchiveTemplate(ctx context.Context, id string) (Template, error) {
	result, err := r.database.ExecContext(ctx, `
		UPDATE agent_templates
		SET status = 'archived', updated_at = datetime('now')
		WHERE id = ?;
	`, id)
	if err != nil {
		return Template{}, err
	}
	if err := requireAffected(result); err != nil {
		return Template{}, err
	}
	return r.GetTemplate(ctx, id)
}

func (r *Repository) CreateSkill(ctx context.Context, skill Skill) (Skill, error) {
	_, err := r.database.ExecContext(ctx, `
		INSERT INTO template_skills (id, template_id, skill_name, skill_path, checksum)
		VALUES (?, ?, ?, ?, ?);
	`, skill.ID, skill.TemplateID, skill.SkillName, skill.SkillPath, skill.Checksum)
	if err != nil {
		if isUniqueSkillNameConstraint(err) {
			return Skill{}, ErrConflict
		}
		return Skill{}, err
	}
	return r.GetSkill(ctx, skill.TemplateID, skill.ID)
}

func (r *Repository) FindSkillByName(ctx context.Context, templateID, skillName string) (Skill, error) {
	var skill Skill
	err := r.database.QueryRowContext(ctx, `
		SELECT id, template_id, skill_name, skill_path, checksum, created_at
		FROM template_skills
		WHERE template_id = ? AND skill_name = ?;
	`, templateID, skillName).Scan(&skill.ID, &skill.TemplateID, &skill.SkillName, &skill.SkillPath, &skill.Checksum, &skill.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Skill{}, ErrSkillNotFound
	}
	if err != nil {
		return Skill{}, err
	}
	return skill, nil
}

func (r *Repository) GetSkill(ctx context.Context, templateID, skillID string) (Skill, error) {
	var skill Skill
	err := r.database.QueryRowContext(ctx, `
		SELECT id, template_id, skill_name, skill_path, checksum, created_at
		FROM template_skills
		WHERE template_id = ? AND id = ?;
	`, templateID, skillID).Scan(&skill.ID, &skill.TemplateID, &skill.SkillName, &skill.SkillPath, &skill.Checksum, &skill.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Skill{}, ErrSkillNotFound
	}
	if err != nil {
		return Skill{}, err
	}
	return skill, nil
}

func (r *Repository) ListSkills(ctx context.Context, templateID string) ([]Skill, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT id, template_id, skill_name, skill_path, checksum, created_at
		FROM template_skills
		WHERE template_id = ?
		ORDER BY skill_name ASC;
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var skill Skill
		if err := rows.Scan(&skill.ID, &skill.TemplateID, &skill.SkillName, &skill.SkillPath, &skill.Checksum, &skill.CreatedAt); err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, rows.Err()
}

func (r *Repository) DeleteSkill(ctx context.Context, templateID, skillID string) error {
	result, err := r.database.ExecContext(ctx, `
		DELETE FROM template_skills
		WHERE template_id = ? AND id = ?;
	`, templateID, skillID)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func isUniqueSkillNameConstraint(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed: template_skills.template_id, template_skills.skill_name")
}
