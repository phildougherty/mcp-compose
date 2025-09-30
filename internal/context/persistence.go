package context

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type PersistenceManager struct {
	db *sql.DB
}

func NewPersistenceManager(dbPath string) (*PersistenceManager, error) {
	if dbPath == "" {
		dbPath = "./mcp-compose-context.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	pm := &PersistenceManager{db: db}

	if err := pm.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return pm, nil
}

func (pm *PersistenceManager) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		type TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		priority INTEGER DEFAULT 0,
		token_count INTEGER NOT NULL,
		last_access TIMESTAMP NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
	CREATE INDEX IF NOT EXISTS idx_messages_last_access ON messages(last_access);
	CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at);
	`

	_, err := pm.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

func (pm *PersistenceManager) Close() error {
	return pm.db.Close()
}

func (pm *PersistenceManager) SaveMessage(conversationID string, msg Message) error {
	tx, err := pm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339Nano)

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO conversations (id, updated_at)
		VALUES (?, ?)
	`, conversationID, now)
	if err != nil {
		return fmt.Errorf("failed to upsert conversation: %w", err)
	}

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO messages (id, conversation_id, type, content, timestamp, priority, token_count, last_access)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, conversationID, msg.Type, msg.Content, msg.Timestamp.Format(time.RFC3339Nano), msg.Priority, msg.TokenCount, msg.LastAccess.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (pm *PersistenceManager) LoadConversation(conversationID string) ([]Message, error) {
	rows, err := pm.db.Query(`
		SELECT id, type, content, timestamp, priority, token_count, last_access
		FROM messages
		WHERE conversation_id = ?
		ORDER BY timestamp ASC
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		var msg Message
		var msgType string
		var timestamp, lastAccess string

		err := rows.Scan(&msg.ID, &msgType, &msg.Content, &timestamp, &msg.Priority, &msg.TokenCount, &lastAccess)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		msg.Type = MessageType(msgType)

		parsedTimestamp, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}
		msg.Timestamp = parsedTimestamp

		parsedLastAccess, err := time.Parse(time.RFC3339Nano, lastAccess)
		if err != nil {
			return nil, fmt.Errorf("failed to parse last_access: %w", err)
		}
		msg.LastAccess = parsedLastAccess

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

func (pm *PersistenceManager) DeleteConversation(conversationID string) error {
	tx, err := pm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM messages WHERE conversation_id = ?`, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	_, err = tx.Exec(`DELETE FROM conversations WHERE id = ?`, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (pm *PersistenceManager) ListConversations() ([]string, error) {
	rows, err := pm.db.Query(`SELECT id FROM conversations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %w", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan conversation id: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating conversations: %w", err)
	}

	return ids, nil
}

func (pm *PersistenceManager) DeleteOldMessages(cutoff time.Time) error {
	tx, err := pm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	cutoffStr := cutoff.Format(time.RFC3339Nano)

	_, err = tx.Exec(`DELETE FROM messages WHERE last_access < ?`, cutoffStr)
	if err != nil {
		return fmt.Errorf("failed to delete old messages: %w", err)
	}

	_, err = tx.Exec(`
		DELETE FROM conversations
		WHERE id NOT IN (SELECT DISTINCT conversation_id FROM messages)
	`)
	if err != nil {
		return fmt.Errorf("failed to delete empty conversations: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	_, err = pm.db.Exec(`VACUUM`)
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	return nil
}

func (pm *PersistenceManager) GetConversationStats(conversationID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var messageCount, totalTokens int
	var oldestTimestamp, newestTimestamp sql.NullString

	err := pm.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(token_count), 0),
			MIN(timestamp),
			MAX(timestamp)
		FROM messages
		WHERE conversation_id = ?
	`, conversationID).Scan(&messageCount, &totalTokens, &oldestTimestamp, &newestTimestamp)

	if err != nil {
		return nil, fmt.Errorf("failed to get conversation stats: %w", err)
	}

	stats["message_count"] = messageCount
	stats["total_tokens"] = totalTokens

	if oldestTimestamp.Valid {
		parsedOldest, err := time.Parse(time.RFC3339Nano, oldestTimestamp.String)
		if err == nil {
			stats["oldest_message"] = parsedOldest
		}
	}
	if newestTimestamp.Valid {
		parsedNewest, err := time.Parse(time.RFC3339Nano, newestTimestamp.String)
		if err == nil {
			stats["newest_message"] = parsedNewest
		}
	}

	return stats, nil
}

func (pm *PersistenceManager) GetMessagesPaginated(conversationID string, offset, limit int) ([]Message, error) {
	rows, err := pm.db.Query(`
		SELECT id, type, content, timestamp, priority, token_count, last_access
		FROM messages
		WHERE conversation_id = ?
		ORDER BY timestamp ASC
		LIMIT ? OFFSET ?
	`, conversationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		var msg Message
		var msgType string
		var timestamp, lastAccess string

		err := rows.Scan(&msg.ID, &msgType, &msg.Content, &timestamp, &msg.Priority, &msg.TokenCount, &lastAccess)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		msg.Type = MessageType(msgType)

		parsedTimestamp, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}
		msg.Timestamp = parsedTimestamp

		parsedLastAccess, err := time.Parse(time.RFC3339Nano, lastAccess)
		if err != nil {
			return nil, fmt.Errorf("failed to parse last_access: %w", err)
		}
		msg.LastAccess = parsedLastAccess

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

func (pm *PersistenceManager) ExportConversation(conversationID string) ([]byte, error) {
	messages, err := pm.LoadConversation(conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation: %w", err)
	}

	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conversation: %w", err)
	}

	return data, nil
}

func (pm *PersistenceManager) ImportConversation(conversationID string, data []byte) error {
	var messages []Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Errorf("failed to unmarshal conversation: %w", err)
	}

	tx, err := pm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM messages WHERE conversation_id = ?`, conversationID)
	if err != nil {
		return fmt.Errorf("failed to clear existing messages: %w", err)
	}

	for _, msg := range messages {
		_, err = tx.Exec(`
			INSERT INTO messages (id, conversation_id, type, content, timestamp, priority, token_count, last_access)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, msg.ID, conversationID, msg.Type, msg.Content, msg.Timestamp.Format(time.RFC3339Nano), msg.Priority, msg.TokenCount, msg.LastAccess.Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("failed to insert message: %w", err)
		}
	}

	now := time.Now().Format(time.RFC3339Nano)

	_, err = tx.Exec(`
		INSERT OR REPLACE INTO conversations (id, updated_at)
		VALUES (?, ?)
	`, conversationID, now)
	if err != nil {
		return fmt.Errorf("failed to upsert conversation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}