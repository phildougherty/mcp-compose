package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type ChatStorage struct {
	db *sql.DB
}

type ChatSession struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model"`
	CreatedAt   time.Time              `json:"created_at"`
	LastUsed    time.Time              `json:"last_used"`
	Title       string                 `json:"title"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Messages    []ChatMessage          `json:"messages,omitempty"`
	MCPServers  []string               `json:"mcp_servers,omitempty"`
}


type SessionStats struct {
	TotalMessages   int     `json:"total_messages"`
	TotalTokens     int     `json:"total_tokens"`
	TotalCost       float64 `json:"total_cost"`
	ToolCallsCount  int     `json:"tool_calls_count"`
}

func NewChatStorage(db *sql.DB) (*ChatStorage, error) {
	storage := &ChatStorage{db: db}
	if err := storage.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return storage, nil
}

func (s *ChatStorage) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			last_used TIMESTAMPTZ DEFAULT NOW(),
			title TEXT DEFAULT 'New Chat',
			metadata JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id TEXT PRIMARY KEY,
			session_id TEXT REFERENCES chat_sessions(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			tool_calls JSONB,
			tool_results JSONB,
			tokens_used INTEGER,
			cost_estimate NUMERIC(10,6),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_id ON chat_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_last_used ON chat_sessions(last_used DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages(created_at)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

func (s *ChatStorage) CreateSession(ctx context.Context, userID, provider, model string) (*ChatSession, error) {
	session := &ChatSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		Provider:  provider,
		Model:     model,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Title:     "New Chat",
		Metadata:  make(map[string]interface{}),
	}

	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `INSERT INTO chat_sessions (id, user_id, provider, model, created_at, last_used, title, metadata)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	result, err := s.db.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.Provider,
		session.Model,
		session.CreatedAt,
		session.LastUsed,
		session.Title,
		metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("session was not inserted into database (0 rows affected)")
	}

	verifyQuery := `SELECT id FROM chat_sessions WHERE id = $1`
	var verifiedID string
	err = s.db.QueryRowContext(ctx, verifyQuery, session.ID).Scan(&verifiedID)
	if err != nil {
		return nil, fmt.Errorf("session insert appeared successful but verification failed: %w", err)
	}

	return session, nil
}

func (s *ChatStorage) GetSession(ctx context.Context, sessionID string) (*ChatSession, error) {
	session := &ChatSession{}
	var metadataJSON []byte

	query := `SELECT id, user_id, provider, model, created_at, last_used, title, metadata
	          FROM chat_sessions WHERE id = $1`

	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.Provider,
		&session.Model,
		&session.CreatedAt,
		&session.LastUsed,
		&session.Title,
		&metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &session.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		if mcpServers, ok := session.Metadata["mcp_servers"].([]interface{}); ok {
			session.MCPServers = make([]string, 0, len(mcpServers))
			for _, server := range mcpServers {
				if serverStr, ok := server.(string); ok {
					session.MCPServers = append(session.MCPServers, serverStr)
				}
			}
		}
	}

	return session, nil
}

func (s *ChatStorage) ListSessions(ctx context.Context, userID string, limit int) ([]*ChatSession, error) {
	if limit <= 0 {
		limit = 50
	}

	var query string
	var rows *sql.Rows
	var err error

	if userID == "" {
		query = `SELECT s.id, s.user_id, s.provider, s.model, s.created_at, s.last_used, s.title, s.metadata
		          FROM chat_sessions s
		          ORDER BY s.last_used DESC
		          LIMIT $1`
		rows, err = s.db.QueryContext(ctx, query, limit)
	} else {
		query = `SELECT s.id, s.user_id, s.provider, s.model, s.created_at, s.last_used, s.title, s.metadata
		          FROM chat_sessions s
		          WHERE s.user_id = $1
		          ORDER BY s.last_used DESC
		          LIMIT $2`
		rows, err = s.db.QueryContext(ctx, query, userID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*ChatSession
	for rows.Next() {
		session := &ChatSession{}
		var metadataJSON []byte

		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.Provider,
			&session.Model,
			&session.CreatedAt,
			&session.LastUsed,
			&session.Title,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &session.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}

			if mcpServers, ok := session.Metadata["mcp_servers"].([]interface{}); ok {
				session.MCPServers = make([]string, 0, len(mcpServers))
				for _, server := range mcpServers {
					if serverStr, ok := server.(string); ok {
						session.MCPServers = append(session.MCPServers, serverStr)
					}
				}
			}
		}

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate sessions: %w", err)
	}

	return sessions, nil
}

func (s *ChatStorage) UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	allowedFields := map[string]bool{
		"title":     true,
		"provider":  true,
		"model":     true,
		"metadata":  true,
		"last_used": true,
	}

	query := "UPDATE chat_sessions SET "
	args := []interface{}{}
	argPos := 1

	for field, value := range updates {
		if !allowedFields[field] {
			continue
		}

		if argPos > 1 {
			query += ", "
		}

		if field == "metadata" {
			metadataJSON, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}
			query += fmt.Sprintf("%s = $%d", field, argPos)
			args = append(args, metadataJSON)
		} else {
			query += fmt.Sprintf("%s = $%d", field, argPos)
			args = append(args, value)
		}

		argPos++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argPos)
	args = append(args, sessionID)

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (s *ChatStorage) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM chat_sessions WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (s *ChatStorage) AddMessage(ctx context.Context, message *ChatMessage) error {
	if message.ID == "" {
		message.ID = uuid.New().String()
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now()
	}

	var toolCallsJSON, toolResultsJSON interface{}
	var err error

	if len(message.ToolCalls) > 0 {
		toolCallsJSON, err = json.Marshal(message.ToolCalls)
		if err != nil {
			return fmt.Errorf("failed to marshal tool calls: %w", err)
		}
	} else {
		toolCallsJSON = nil
	}

	if len(message.ToolResults) > 0 {
		toolResultsJSON, err = json.Marshal(message.ToolResults)
		if err != nil {
			return fmt.Errorf("failed to marshal tool results: %w", err)
		}
	} else {
		toolResultsJSON = nil
	}

	query := `INSERT INTO chat_messages (id, session_id, role, content, tool_calls, tool_results, tokens_used, cost_estimate, is_automated, from_task_run_id, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	var fromTaskRunID interface{}
	if message.FromTaskRunID != "" {
		fromTaskRunID = message.FromTaskRunID
	} else {
		fromTaskRunID = nil
	}

	_, err = s.db.ExecContext(ctx, query,
		message.ID,
		message.SessionID,
		message.Role,
		message.Content,
		toolCallsJSON,
		toolResultsJSON,
		message.TokensUsed,
		message.CostEstimate,
		message.IsAutomated,
		fromTaskRunID,
		message.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	updateQuery := `UPDATE chat_sessions SET last_used = $1 WHERE id = $2`
	_, err = s.db.ExecContext(ctx, updateQuery, time.Now(), message.SessionID)
	if err != nil {
		return fmt.Errorf("failed to update session last_used: %w", err)
	}

	return nil
}

func (s *ChatStorage) GetMessages(ctx context.Context, sessionID string, limit int) ([]*ChatMessage, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, session_id, role, content, tool_calls, tool_results, tokens_used, cost_estimate, is_automated, from_task_run_id, created_at
	          FROM chat_messages
	          WHERE session_id = $1
	          ORDER BY created_at ASC
	          LIMIT $2`

	rows, err := s.db.QueryContext(ctx, query, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []*ChatMessage
	for rows.Next() {
		message := &ChatMessage{}
		var toolCallsJSON, toolResultsJSON []byte
		var tokensUsed sql.NullInt64
		var costEstimate sql.NullFloat64
		var isAutomated sql.NullBool
		var fromTaskRunID sql.NullString

		err := rows.Scan(
			&message.ID,
			&message.SessionID,
			&message.Role,
			&message.Content,
			&toolCallsJSON,
			&toolResultsJSON,
			&tokensUsed,
			&costEstimate,
			&isAutomated,
			&fromTaskRunID,
			&message.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if tokensUsed.Valid {
			message.TokensUsed = int(tokensUsed.Int64)
		}
		if costEstimate.Valid {
			message.CostEstimate = costEstimate.Float64
		}
		if isAutomated.Valid {
			message.IsAutomated = isAutomated.Bool
		}
		if fromTaskRunID.Valid {
			message.FromTaskRunID = fromTaskRunID.String
		}

		if len(toolCallsJSON) > 0 {
			if err := json.Unmarshal(toolCallsJSON, &message.ToolCalls); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool calls: %w", err)
			}
		}

		if len(toolResultsJSON) > 0 {
			if err := json.Unmarshal(toolResultsJSON, &message.ToolResults); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool results: %w", err)
			}
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate messages: %w", err)
	}

	return messages, nil
}

func (s *ChatStorage) GetSessionStats(ctx context.Context, sessionID string) (*SessionStats, error) {
	stats := &SessionStats{}

	query := `SELECT
	          COUNT(*) as total_messages,
	          COALESCE(SUM(tokens_used), 0) as total_tokens,
	          COALESCE(SUM(cost_estimate), 0) as total_cost,
	          COALESCE(SUM(CASE WHEN tool_calls IS NOT NULL AND tool_calls != 'null' THEN 1 ELSE 0 END), 0) as tool_calls_count
	          FROM chat_messages
	          WHERE session_id = $1`

	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&stats.TotalMessages,
		&stats.TotalTokens,
		&stats.TotalCost,
		&stats.ToolCallsCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get session stats: %w", err)
	}

	return stats, nil
}

func (s *ChatStorage) CleanupOldSessions(ctx context.Context, daysOld int) (int64, error) {
	if daysOld <= 0 {
		daysOld = 30
	}

	query := `DELETE FROM chat_sessions WHERE last_used < NOW() - INTERVAL '$1 days'`

	result, err := s.db.ExecContext(ctx, query, daysOld)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

func (s *ChatStorage) SetSessionMCPServers(ctx context.Context, sessionID string, mcpServers []string) error {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	session.Metadata["mcp_servers"] = mcpServers

	return s.UpdateSession(ctx, sessionID, map[string]interface{}{
		"metadata": session.Metadata,
	})
}

func (s *ChatStorage) GetSessionMCPServers(ctx context.Context, sessionID string) ([]string, error) {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return session.MCPServers, nil
}

func (s *ChatStorage) IncrementUnreadCount(ctx context.Context, sessionID string) error {
	query := `UPDATE chat_sessions SET unread_message_count = unread_message_count + 1 WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to increment unread count: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (s *ChatStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}

	return nil
}