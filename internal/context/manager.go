package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkoukk/tiktoken-go"
)

type MessageType string

const (
	MessageTypeSystem     MessageType = "system"
	MessageTypeUser       MessageType = "user"
	MessageTypeAssistant  MessageType = "assistant"
	MessageTypeToolUse    MessageType = "tool_use"
	MessageTypeToolResult MessageType = "tool_result"
)

type Message struct {
	ID         string      `json:"id"`
	Type       MessageType `json:"type"`
	Content    string      `json:"content"`
	Timestamp  time.Time   `json:"timestamp"`
	Priority   int         `json:"priority"`
	TokenCount int         `json:"token_count"`
	LastAccess time.Time   `json:"last_access"`
}

type Config struct {
	MaxTokens           int
	Model               string
	TruncationStrategy  TruncationStrategy
	PersistenceEnabled  bool
	DatabasePath        string
	ContextTTL          time.Duration
	VacuumInterval      time.Duration
}

type Manager struct {
	config      Config
	messages    map[string][]Message
	mu          sync.RWMutex
	tokenizer   *tiktoken.Tiktoken
	persistence *PersistenceManager
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewManager(cfg Config) (*Manager, error) {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 32000
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4"
	}
	if cfg.ContextTTL == 0 {
		cfg.ContextTTL = 24 * time.Hour
	}
	if cfg.VacuumInterval == 0 {
		cfg.VacuumInterval = 1 * time.Hour
	}

	encoding := "cl100k_base"
	tokenizer, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config:    cfg,
		messages:  make(map[string][]Message),
		tokenizer: tokenizer,
		ctx:       ctx,
		cancel:    cancel,
	}

	if cfg.PersistenceEnabled {
		persistence, err := NewPersistenceManager(cfg.DatabasePath)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create persistence manager: %w", err)
		}
		m.persistence = persistence

		m.wg.Add(1)
		go m.vacuumLoop()
	}

	return m, nil
}

func (m *Manager) Close() error {
	m.cancel()
	m.wg.Wait()

	if m.persistence != nil {
		return m.persistence.Close()
	}

	return nil
}

func (m *Manager) AddMessage(conversationID string, msg Message) error {
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	msg.LastAccess = time.Now()

	msg.TokenCount = m.CountTokens(msg.Content)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages[conversationID] = append(m.messages[conversationID], msg)

	if err := m.truncateIfNeeded(conversationID); err != nil {
		return fmt.Errorf("failed to truncate: %w", err)
	}

	if m.persistence != nil {
		if err := m.persistence.SaveMessage(conversationID, msg); err != nil {
			return fmt.Errorf("failed to persist message: %w", err)
		}
	}

	return nil
}

func (m *Manager) GetMessages(conversationID string) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages, ok := m.messages[conversationID]
	if !ok {
		if m.persistence != nil {
			var err error
			messages, err = m.persistence.LoadConversation(conversationID)
			if err != nil {
				return nil, fmt.Errorf("failed to load conversation: %w", err)
			}
			m.messages[conversationID] = messages
		} else {
			return []Message{}, nil
		}
	}

	now := time.Now()
	for i := range messages {
		messages[i].LastAccess = now
	}

	result := make([]Message, len(messages))
	copy(result, messages)

	return result, nil
}

func (m *Manager) CountTokens(text string) int {
	tokens := m.tokenizer.Encode(text, nil, nil)
	return len(tokens)
}

func (m *Manager) GetTotalTokens(conversationID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages := m.messages[conversationID]
	total := 0
	for _, msg := range messages {
		total += msg.TokenCount
	}

	return total
}

func (m *Manager) truncateIfNeeded(conversationID string) error {
	total := 0
	messages := m.messages[conversationID]

	for _, msg := range messages {
		total += msg.TokenCount
	}

	if total <= m.config.MaxTokens {
		return nil
	}

	truncated, err := ApplyTruncation(m.config.TruncationStrategy, messages, m.config.MaxTokens)
	if err != nil {
		return err
	}

	m.messages[conversationID] = truncated

	return nil
}

func (m *Manager) ClearConversation(conversationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.messages, conversationID)

	if m.persistence != nil {
		if err := m.persistence.DeleteConversation(conversationID); err != nil {
			return fmt.Errorf("failed to delete conversation: %w", err)
		}
	}

	return nil
}

func (m *Manager) ListConversations() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.messages))
	for id := range m.messages {
		ids = append(ids, id)
	}

	if m.persistence != nil {
		persistedIDs, err := m.persistence.ListConversations()
		if err != nil {
			return nil, fmt.Errorf("failed to list conversations: %w", err)
		}

		seen := make(map[string]bool)
		for _, id := range ids {
			seen[id] = true
		}

		for _, id := range persistedIDs {
			if !seen[id] {
				ids = append(ids, id)
			}
		}
	}

	return ids, nil
}

func (m *Manager) vacuumLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.VacuumInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.vacuum(); err != nil {
				continue
			}
		}
	}
}

func (m *Manager) vacuum() error {
	if m.persistence == nil {
		return nil
	}

	cutoff := time.Now().Add(-m.config.ContextTTL)
	return m.persistence.DeleteOldMessages(cutoff)
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}