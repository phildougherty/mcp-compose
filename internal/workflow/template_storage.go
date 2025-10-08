package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type TemplateStorage struct {
	db *sql.DB
}

func NewTemplateStorage(db *sql.DB) (*TemplateStorage, error) {
	storage := &TemplateStorage{db: db}
	if err := storage.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize template tables: %w", err)
	}

	return storage, nil
}

func (s *TemplateStorage) initTables() error {
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS templates`,
		`CREATE TABLE IF NOT EXISTS templates.workflow_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			category TEXT NOT NULL,
			author TEXT NOT NULL,
			thumbnail TEXT,
			tags TEXT[] DEFAULT '{}',
			version TEXT NOT NULL,
			downloads INTEGER DEFAULT 0,
			rating DOUBLE PRECISION DEFAULT 0.0,
			workflow_definition JSONB NOT NULL,
			required_servers TEXT[] DEFAULT '{}',
			estimated_cost TEXT,
			difficulty TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_templates_category ON templates.workflow_templates(category)`,
		`CREATE INDEX IF NOT EXISTS idx_templates_difficulty ON templates.workflow_templates(difficulty)`,
		`CREATE INDEX IF NOT EXISTS idx_templates_downloads ON templates.workflow_templates(downloads DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_templates_rating ON templates.workflow_templates(rating DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_templates_created_at ON templates.workflow_templates(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_templates_tags ON templates.workflow_templates USING GIN(tags)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

func (s *TemplateStorage) CreateTemplate(ctx context.Context, template *Template) error {
	if template.ID == "" {
		template.ID = uuid.New().String()
	}
	if template.CreatedAt.IsZero() {
		template.CreatedAt = time.Now()
	}
	template.UpdatedAt = time.Now()

	query := `INSERT INTO templates.workflow_templates
		(id, name, description, category, author, thumbnail, tags, version, downloads, rating,
		workflow_definition, required_servers, estimated_cost, difficulty, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	_, err := s.db.ExecContext(ctx, query,
		template.ID,
		template.Name,
		template.Description,
		template.Category,
		template.Author,
		template.Thumbnail,
		pqStringArray(template.Tags),
		template.Version,
		template.Downloads,
		template.Rating,
		template.WorkflowDefinition,
		pqStringArray(template.RequiredServers),
		template.EstimatedCost,
		template.Difficulty,
		template.CreatedAt,
		template.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	return nil
}

func (s *TemplateStorage) GetTemplate(ctx context.Context, templateID string) (*Template, error) {
	template := &Template{}

	query := `SELECT id, name, description, category, author, thumbnail, tags, version,
		downloads, rating, workflow_definition, required_servers, estimated_cost, difficulty,
		created_at, updated_at
		FROM templates.workflow_templates WHERE id = $1`

	err := s.db.QueryRowContext(ctx, query, templateID).Scan(
		&template.ID,
		&template.Name,
		&template.Description,
		&template.Category,
		&template.Author,
		&template.Thumbnail,
		pq.Array(&template.Tags),
		&template.Version,
		&template.Downloads,
		&template.Rating,
		&template.WorkflowDefinition,
		pq.Array(&template.RequiredServers),
		&template.EstimatedCost,
		&template.Difficulty,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("template not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return template, nil
}

func (s *TemplateStorage) ListTemplates(ctx context.Context, filter TemplateFilter) ([]*Template, error) {
	query := `SELECT id, name, description, category, author, thumbnail, tags, version,
		downloads, rating, workflow_definition, required_servers, estimated_cost, difficulty,
		created_at, updated_at
		FROM templates.workflow_templates`

	var conditions []string
	var args []interface{}
	argCount := 1

	if filter.Category != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argCount))
		args = append(args, filter.Category)
		argCount++
	}

	if filter.Difficulty != "" {
		conditions = append(conditions, fmt.Sprintf("difficulty = $%d", argCount))
		args = append(args, filter.Difficulty)
		argCount++
	}

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR description ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+filter.Search+"%")
		argCount++
	}

	if len(filter.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("tags && $%d", argCount))
		args = append(args, pq.Array(filter.Tags))
		argCount++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	switch filter.SortBy {
	case SortByPopularity:
		query += " ORDER BY downloads DESC"
	case SortByRating:
		query += " ORDER BY rating DESC"
	case SortByRecent:
		query += " ORDER BY created_at DESC"
	default:
		query += " ORDER BY created_at DESC"
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, filter.Limit)
	argCount++

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	var templates []*Template
	for rows.Next() {
		template := &Template{}

		err := rows.Scan(
			&template.ID,
			&template.Name,
			&template.Description,
			&template.Category,
			&template.Author,
			&template.Thumbnail,
			pq.Array(&template.Tags),
			&template.Version,
			&template.Downloads,
			&template.Rating,
			&template.WorkflowDefinition,
			pq.Array(&template.RequiredServers),
			&template.EstimatedCost,
			&template.Difficulty,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template: %w", err)
		}

		templates = append(templates, template)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate templates: %w", err)
	}

	return templates, nil
}

func (s *TemplateStorage) UpdateTemplate(ctx context.Context, template *Template) error {
	template.UpdatedAt = time.Now()

	query := `UPDATE templates.workflow_templates
		SET name = $1, description = $2, category = $3, author = $4, thumbnail = $5,
		tags = $6, version = $7, rating = $8, workflow_definition = $9,
		required_servers = $10, estimated_cost = $11, difficulty = $12, updated_at = $13
		WHERE id = $14`

	result, err := s.db.ExecContext(ctx, query,
		template.Name,
		template.Description,
		template.Category,
		template.Author,
		template.Thumbnail,
		pqStringArray(template.Tags),
		template.Version,
		template.Rating,
		template.WorkflowDefinition,
		pqStringArray(template.RequiredServers),
		template.EstimatedCost,
		template.Difficulty,
		template.UpdatedAt,
		template.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("template not found")
	}

	return nil
}

func (s *TemplateStorage) DeleteTemplate(ctx context.Context, templateID string) error {
	query := `DELETE FROM templates.workflow_templates WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, templateID)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("template not found")
	}

	return nil
}

func (s *TemplateStorage) SearchTemplates(ctx context.Context, query string) ([]*Template, error) {
	return s.ListTemplates(ctx, TemplateFilter{
		Search: query,
		Limit:  50,
	})
}

func (s *TemplateStorage) IncrementDownloads(ctx context.Context, templateID string) error {
	query := `UPDATE templates.workflow_templates SET downloads = downloads + 1 WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, templateID)
	if err != nil {
		return fmt.Errorf("failed to increment downloads: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("template not found")
	}

	return nil
}

func (s *TemplateStorage) GetCategoryCounts(ctx context.Context) ([]CategoryCount, error) {
	query := `SELECT category, COUNT(*) as count
		FROM templates.workflow_templates
		GROUP BY category
		ORDER BY count DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get category counts: %w", err)
	}
	defer rows.Close()

	var counts []CategoryCount
	for rows.Next() {
		var cc CategoryCount
		err := rows.Scan(&cc.Category, &cc.Count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category count: %w", err)
		}
		counts = append(counts, cc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate category counts: %w", err)
	}

	return counts, nil
}

func (s *TemplateStorage) InstallTemplate(ctx context.Context, workflowStorage *Storage, templateID string, parameters map[string]interface{}) (string, error) {
	template, err := s.GetTemplate(ctx, templateID)
	if err != nil {
		return "", fmt.Errorf("failed to get template: %w", err)
	}

	var workflowDef Workflow
	if err := json.Unmarshal(template.WorkflowDefinition, &workflowDef); err != nil {
		return "", fmt.Errorf("failed to unmarshal workflow definition: %w", err)
	}

	workflowDef.ID = uuid.New().String()
	workflowDef.Name = template.Name + " (from template)"
	workflowDef.Description = template.Description
	workflowDef.CreatedAt = time.Now()
	workflowDef.UpdatedAt = time.Now()

	if workflowDef.Metadata.Tags == nil {
		workflowDef.Metadata.Tags = []string{}
	}
	workflowDef.Metadata.Tags = append(workflowDef.Metadata.Tags, "from-template")
	workflowDef.Metadata.Category = template.Category

	if workflowDef.Metadata.CustomData == nil {
		workflowDef.Metadata.CustomData = make(map[string]interface{})
	}
	workflowDef.Metadata.CustomData["template_id"] = template.ID
	workflowDef.Metadata.CustomData["template_version"] = template.Version

	if err := workflowStorage.CreateWorkflow(ctx, &workflowDef); err != nil {
		return "", fmt.Errorf("failed to create workflow from template: %w", err)
	}

	if err := s.IncrementDownloads(ctx, templateID); err != nil {
		return "", fmt.Errorf("failed to increment download count: %w", err)
	}

	return workflowDef.ID, nil
}

func (s *TemplateStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}

	return nil
}

func pqStringArray(arr []string) pq.StringArray {
	if arr == nil {
		return pq.StringArray{}
	}

	return pq.StringArray(arr)
}
