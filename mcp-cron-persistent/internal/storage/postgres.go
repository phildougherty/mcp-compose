package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"mcp-cron-persistent/internal/model"

	_ "github.com/lib/pq"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(connectionURL string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", connectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	_, err = db.ExecContext(ctx, "SET search_path TO task_scheduler, public")
	if err != nil {
		return nil, fmt.Errorf("failed to set search path: %w", err)
	}

	return &PostgresStorage{db: db}, nil
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

func (s *PostgresStorage) CreateTask(ctx context.Context, task *model.Task) error {
	query := `
        INSERT INTO task_scheduler.scheduler_tasks (
            id, name, description, type, enabled, command, prompt, schedule,
            timezone, chat_session_id, output_to_chat, inherit_session_context,
            provider, model, mcp_servers, status, trigger_type, is_agent,
            user_id, created_by, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
            $16, $17, $18, $19, $20, $21, $22
        )`

	mcpServersJSON, err := json.Marshal(task.MCPServers)
	if err != nil {
		return fmt.Errorf("failed to marshal mcp_servers: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query,
		task.ID, task.Name, task.Description, task.Type, task.Enabled,
		task.Command, task.Prompt, task.Schedule, task.Timezone,
		task.ChatSessionID, task.OutputToChat, task.InheritSessionContext,
		task.Provider, task.Model, mcpServersJSON, task.Status,
		task.TriggerType, task.IsAgent, task.UserID, task.CreatedBy,
		task.CreatedAt, task.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert task: %w", err)
	}

	return nil
}

func (s *PostgresStorage) GetTask(ctx context.Context, taskID string) (*model.Task, error) {
	query := `
        SELECT id, name, description, type, enabled, command, prompt, schedule,
               timezone, chat_session_id, output_to_chat, inherit_session_context,
               provider, model, mcp_servers, status, last_run, next_run,
               trigger_type, is_agent, agent_personality, user_id, created_at, updated_at
        FROM task_scheduler.scheduler_tasks
        WHERE id = $1`

	var task model.Task
	var mcpServersJSON []byte
	var lastRun, nextRun sql.NullTime
	var chatSessionID sql.NullString

	err := s.db.QueryRowContext(ctx, query, taskID).Scan(
		&task.ID, &task.Name, &task.Description, &task.Type, &task.Enabled,
		&task.Command, &task.Prompt, &task.Schedule, &task.Timezone,
		&chatSessionID, &task.OutputToChat, &task.InheritSessionContext,
		&task.Provider, &task.Model, &mcpServersJSON, &task.Status,
		&lastRun, &nextRun, &task.TriggerType, &task.IsAgent,
		&task.AgentPersonality, &task.UserID, &task.CreatedAt, &task.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query task: %w", err)
	}

	if chatSessionID.Valid {
		task.ChatSessionID = chatSessionID.String
	}

	if lastRun.Valid {
		task.LastRun = lastRun.Time
	}

	if nextRun.Valid {
		task.NextRun = nextRun.Time
	}

	json.Unmarshal(mcpServersJSON, &task.MCPServers)

	return &task, nil
}

func (s *PostgresStorage) ListTasksBySession(ctx context.Context, sessionID string) ([]*model.Task, error) {
	query := `
        SELECT id, name, description, type, enabled, schedule, status,
               last_run, next_run, is_agent, user_id, created_at
        FROM task_scheduler.scheduler_tasks
        WHERE chat_session_id = $1
        ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		var task model.Task
		var lastRun, nextRun sql.NullTime

		err := rows.Scan(
			&task.ID, &task.Name, &task.Description, &task.Type,
			&task.Enabled, &task.Schedule, &task.Status,
			&lastRun, &nextRun, &task.IsAgent, &task.UserID, &task.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		if lastRun.Valid {
			task.LastRun = lastRun.Time
		}
		if nextRun.Valid {
			task.NextRun = nextRun.Time
		}

		tasks = append(tasks, &task)
	}

	return tasks, nil
}

func (s *PostgresStorage) RecordTaskRun(ctx context.Context, run *model.TaskRun) error {
	query := `
        INSERT INTO task_scheduler.scheduler_task_runs (
            id, task_id, started_at, finished_at, output, error,
            exit_code, status, posted_to_chat, chat_message_id,
            triggered_by, tokens_used, cost_estimate
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := s.db.ExecContext(ctx, query,
		run.ID, run.TaskID, run.StartedAt, run.FinishedAt,
		run.Output, run.Error, run.ExitCode, run.Status,
		run.PostedToChat, run.ChatMessageID, run.TriggeredBy,
		run.TokensUsed, run.CostEstimate,
	)

	if err != nil {
		return fmt.Errorf("failed to record task run: %w", err)
	}

	return nil
}

func (s *PostgresStorage) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
}

func (s *PostgresStorage) ListTasksByUser(ctx context.Context, userID string, includeDisabled bool) ([]*model.Task, error) {
	query := `
        SELECT id, name, description, type, enabled, schedule, status,
               last_run, next_run, is_agent, created_at
        FROM task_scheduler.scheduler_tasks
        WHERE user_id = $1`

	if !includeDisabled {
		query += ` AND enabled = true`
	}

	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		var task model.Task
		var lastRun, nextRun sql.NullTime

		err := rows.Scan(
			&task.ID, &task.Name, &task.Description, &task.Type,
			&task.Enabled, &task.Schedule, &task.Status,
			&lastRun, &nextRun, &task.IsAgent, &task.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		if lastRun.Valid {
			task.LastRun = lastRun.Time
		}
		if nextRun.Valid {
			task.NextRun = nextRun.Time
		}

		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}
