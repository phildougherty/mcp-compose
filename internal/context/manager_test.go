package context

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	cfg := Config{
		MaxTokens:          1000,
		Model:              "gpt-4",
		TruncationStrategy: TruncationOldest,
		PersistenceEnabled: false,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, manager)
	defer manager.Close()

	assert.Equal(t, 1000, manager.config.MaxTokens)
	assert.Equal(t, "gpt-4", manager.config.Model)
}

func TestNewManagerDefaults(t *testing.T) {
	cfg := Config{}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, manager)
	defer manager.Close()

	assert.Equal(t, 32000, manager.config.MaxTokens)
	assert.Equal(t, "gpt-4", manager.config.Model)
	assert.Equal(t, 24*time.Hour, manager.config.ContextTTL)
	assert.Equal(t, 1*time.Hour, manager.config.VacuumInterval)
}

func TestAddMessage(t *testing.T) {
	cfg := Config{
		MaxTokens:          1000,
		TruncationStrategy: TruncationOldest,
		PersistenceEnabled: false,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	msg := Message{
		Type:     MessageTypeUser,
		Content:  "Hello, world!",
		Priority: 5,
	}

	err = manager.AddMessage("conv1", msg)
	require.NoError(t, err)

	messages, err := manager.GetMessages("conv1")
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, MessageTypeUser, messages[0].Type)
	assert.Equal(t, "Hello, world!", messages[0].Content)
	assert.Equal(t, 5, messages[0].Priority)
	assert.Greater(t, messages[0].TokenCount, 0)
	assert.NotEmpty(t, messages[0].ID)
}

func TestAddMultipleMessages(t *testing.T) {
	cfg := Config{
		MaxTokens:          1000,
		TruncationStrategy: TruncationOldest,
		PersistenceEnabled: false,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	messages := []Message{
		{Type: MessageTypeSystem, Content: "You are a helpful assistant."},
		{Type: MessageTypeUser, Content: "What is the capital of France?"},
		{Type: MessageTypeAssistant, Content: "The capital of France is Paris."},
	}

	for _, msg := range messages {
		err := manager.AddMessage("conv1", msg)
		require.NoError(t, err)
	}

	retrieved, err := manager.GetMessages("conv1")
	require.NoError(t, err)
	require.Len(t, retrieved, 3)

	assert.Equal(t, MessageTypeSystem, retrieved[0].Type)
	assert.Equal(t, MessageTypeUser, retrieved[1].Type)
	assert.Equal(t, MessageTypeAssistant, retrieved[2].Type)
}

func TestCountTokens(t *testing.T) {
	cfg := Config{
		MaxTokens: 1000,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	text := "Hello, world!"
	count := manager.CountTokens(text)
	assert.Greater(t, count, 0)
	assert.Less(t, count, 10)
}

func TestGetTotalTokens(t *testing.T) {
	cfg := Config{
		MaxTokens:          1000,
		TruncationStrategy: TruncationOldest,
		PersistenceEnabled: false,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	messages := []Message{
		{Type: MessageTypeUser, Content: "Hello"},
		{Type: MessageTypeAssistant, Content: "Hi there!"},
	}

	for _, msg := range messages {
		err := manager.AddMessage("conv1", msg)
		require.NoError(t, err)
	}

	total := manager.GetTotalTokens("conv1")
	assert.Greater(t, total, 0)
}

func TestClearConversation(t *testing.T) {
	cfg := Config{
		MaxTokens:          1000,
		TruncationStrategy: TruncationOldest,
		PersistenceEnabled: false,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	msg := Message{
		Type:    MessageTypeUser,
		Content: "Test message",
	}

	err = manager.AddMessage("conv1", msg)
	require.NoError(t, err)

	messages, err := manager.GetMessages("conv1")
	require.NoError(t, err)
	require.Len(t, messages, 1)

	err = manager.ClearConversation("conv1")
	require.NoError(t, err)

	messages, err = manager.GetMessages("conv1")
	require.NoError(t, err)
	require.Len(t, messages, 0)
}

func TestListConversations(t *testing.T) {
	cfg := Config{
		MaxTokens:          1000,
		TruncationStrategy: TruncationOldest,
		PersistenceEnabled: false,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	conversationIDs := []string{"conv1", "conv2", "conv3"}

	for _, id := range conversationIDs {
		msg := Message{
			Type:    MessageTypeUser,
			Content: "Test message",
		}
		err := manager.AddMessage(id, msg)
		require.NoError(t, err)
	}

	ids, err := manager.ListConversations()
	require.NoError(t, err)
	assert.Len(t, ids, 3)

	for _, id := range conversationIDs {
		assert.Contains(t, ids, id)
	}
}

func TestTruncationWhenExceedingMaxTokens(t *testing.T) {
	cfg := Config{
		MaxTokens:          50,
		TruncationStrategy: TruncationOldest,
		PersistenceEnabled: false,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	messages := []Message{
		{Type: MessageTypeSystem, Content: "You are helpful."},
		{Type: MessageTypeUser, Content: "Tell me a very long story about a knight who goes on adventures."},
		{Type: MessageTypeAssistant, Content: "Once upon a time..."},
		{Type: MessageTypeUser, Content: "Continue the story please."},
	}

	for _, msg := range messages {
		err := manager.AddMessage("conv1", msg)
		require.NoError(t, err)
	}

	total := manager.GetTotalTokens("conv1")
	assert.LessOrEqual(t, total, cfg.MaxTokens)

	retrieved, err := manager.GetMessages("conv1")
	require.NoError(t, err)

	systemCount := 0
	for _, msg := range retrieved {
		if msg.Type == MessageTypeSystem {
			systemCount++
		}
	}
	assert.Greater(t, systemCount, 0)
}

func TestPersistence(t *testing.T) {
	dbPath := "./test_context.db"
	defer os.Remove(dbPath)

	cfg := Config{
		MaxTokens:          1000,
		TruncationStrategy: TruncationOldest,
		PersistenceEnabled: true,
		DatabasePath:       dbPath,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)

	msg := Message{
		Type:    MessageTypeUser,
		Content: "Test persistence",
	}

	err = manager.AddMessage("conv1", msg)
	require.NoError(t, err)

	manager.Close()

	manager2, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager2.Close()

	messages, err := manager2.GetMessages("conv1")
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, "Test persistence", messages[0].Content)
}

func TestConcurrentAccess(t *testing.T) {
	cfg := Config{
		MaxTokens:          1000,
		TruncationStrategy: TruncationOldest,
		PersistenceEnabled: false,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			msg := Message{
				Type:    MessageTypeUser,
				Content: "Concurrent test message",
			}
			err := manager.AddMessage("conv1", msg)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	messages, err := manager.GetMessages("conv1")
	require.NoError(t, err)
	assert.Len(t, messages, 10)
}