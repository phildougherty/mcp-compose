package context

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestMessages() []Message {
	now := time.Now()
	return []Message{
		{
			ID:         "msg1",
			Type:       MessageTypeSystem,
			Content:    "You are a helpful assistant.",
			Timestamp:  now.Add(-10 * time.Minute),
			Priority:   10,
			TokenCount: 7,
			LastAccess: now.Add(-5 * time.Minute),
		},
		{
			ID:         "msg2",
			Type:       MessageTypeUser,
			Content:    "Hello!",
			Timestamp:  now.Add(-9 * time.Minute),
			Priority:   5,
			TokenCount: 3,
			LastAccess: now.Add(-2 * time.Minute),
		},
		{
			ID:         "msg3",
			Type:       MessageTypeAssistant,
			Content:    "Hi! How can I help you today?",
			Timestamp:  now.Add(-8 * time.Minute),
			Priority:   5,
			TokenCount: 8,
			LastAccess: now.Add(-1 * time.Minute),
		},
		{
			ID:         "msg4",
			Type:       MessageTypeUser,
			Content:    "Tell me about the weather.",
			Timestamp:  now.Add(-7 * time.Minute),
			Priority:   5,
			TokenCount: 6,
			LastAccess: now.Add(-10 * time.Second),
		},
		{
			ID:         "msg5",
			Type:       MessageTypeToolUse,
			Content:    "get_weather(location='NYC')",
			Timestamp:  now.Add(-6 * time.Minute),
			Priority:   3,
			TokenCount: 5,
			LastAccess: now.Add(-3 * time.Minute),
		},
		{
			ID:         "msg6",
			Type:       MessageTypeToolResult,
			Content:    "Temperature: 72F, Sunny",
			Timestamp:  now.Add(-5 * time.Minute),
			Priority:   3,
			TokenCount: 6,
			LastAccess: now.Add(-4 * time.Minute),
		},
	}
}

func TestTruncateOldest(t *testing.T) {
	messages := createTestMessages()
	maxTokens := 20

	result, err := truncateOldest(messages, maxTokens)
	require.NoError(t, err)

	totalTokens := 0
	for _, msg := range result {
		totalTokens += msg.TokenCount
	}
	assert.LessOrEqual(t, totalTokens, maxTokens)

	systemPresent := false
	for _, msg := range result {
		if msg.Type == MessageTypeSystem {
			systemPresent = true
			break
		}
	}
	assert.True(t, systemPresent, "System message should be preserved")

	for i := 1; i < len(result); i++ {
		assert.True(t, result[i].Timestamp.After(result[i-1].Timestamp) || result[i].Timestamp.Equal(result[i-1].Timestamp))
	}
}

func TestTruncateLRU(t *testing.T) {
	messages := createTestMessages()
	maxTokens := 20

	result, err := truncateLRU(messages, maxTokens)
	require.NoError(t, err)

	totalTokens := 0
	for _, msg := range result {
		totalTokens += msg.TokenCount
	}
	assert.LessOrEqual(t, totalTokens, maxTokens)

	systemPresent := false
	for _, msg := range result {
		if msg.Type == MessageTypeSystem {
			systemPresent = true
			break
		}
	}
	assert.True(t, systemPresent, "System message should be preserved")

	recentlyAccessedPresent := false
	for _, msg := range result {
		if msg.ID == "msg4" {
			recentlyAccessedPresent = true
			break
		}
	}
	assert.True(t, recentlyAccessedPresent, "Most recently accessed message should be present")
}

func TestTruncateByType(t *testing.T) {
	messages := createTestMessages()
	maxTokens := 20

	result, err := truncateByType(messages, maxTokens)
	require.NoError(t, err)

	totalTokens := 0
	for _, msg := range result {
		totalTokens += msg.TokenCount
	}
	assert.LessOrEqual(t, totalTokens, maxTokens)

	systemPresent := false
	for _, msg := range result {
		if msg.Type == MessageTypeSystem {
			systemPresent = true
			break
		}
	}
	assert.True(t, systemPresent, "System message should be preserved")

	userCount := 0
	assistantCount := 0
	toolCount := 0
	for _, msg := range result {
		switch msg.Type {
		case MessageTypeUser:
			userCount++
		case MessageTypeAssistant:
			assistantCount++
		case MessageTypeToolUse, MessageTypeToolResult:
			toolCount++
		}
	}

	if userCount > 0 {
		assert.GreaterOrEqual(t, userCount, assistantCount, "User messages should be prioritized over assistant")
	}
	if assistantCount > 0 {
		assert.GreaterOrEqual(t, assistantCount, toolCount, "Assistant messages should be prioritized over tool messages")
	}
}

func TestTruncateByPriority(t *testing.T) {
	messages := []Message{
		{
			ID:         "msg1",
			Type:       MessageTypeSystem,
			Content:    "System message",
			Timestamp:  time.Now().Add(-10 * time.Minute),
			Priority:   10,
			TokenCount: 5,
		},
		{
			ID:         "msg2",
			Type:       MessageTypeUser,
			Content:    "High priority",
			Timestamp:  time.Now().Add(-9 * time.Minute),
			Priority:   9,
			TokenCount: 5,
		},
		{
			ID:         "msg3",
			Type:       MessageTypeUser,
			Content:    "Medium priority",
			Timestamp:  time.Now().Add(-8 * time.Minute),
			Priority:   5,
			TokenCount: 5,
		},
		{
			ID:         "msg4",
			Type:       MessageTypeUser,
			Content:    "Low priority",
			Timestamp:  time.Now().Add(-7 * time.Minute),
			Priority:   1,
			TokenCount: 5,
		},
	}

	maxTokens := 12

	result, err := truncateByPriority(messages, maxTokens)
	require.NoError(t, err)

	totalTokens := 0
	for _, msg := range result {
		totalTokens += msg.TokenCount
	}
	assert.LessOrEqual(t, totalTokens, maxTokens)

	highPriorityPresent := false
	lowPriorityPresent := false
	for _, msg := range result {
		if msg.Priority >= 9 {
			highPriorityPresent = true
		}
		if msg.Priority <= 1 {
			lowPriorityPresent = true
		}
	}
	assert.True(t, highPriorityPresent, "High priority messages should be preserved")
	assert.False(t, lowPriorityPresent, "Low priority messages should be truncated")
}

func TestTruncateIntelligent(t *testing.T) {
	messages := createTestMessages()
	maxTokens := 20

	result, err := truncateIntelligent(messages, maxTokens)
	require.NoError(t, err)

	totalTokens := 0
	for _, msg := range result {
		totalTokens += msg.TokenCount
	}
	assert.LessOrEqual(t, totalTokens, maxTokens)

	systemPresent := false
	for _, msg := range result {
		if msg.Type == MessageTypeSystem {
			systemPresent = true
			break
		}
	}
	assert.True(t, systemPresent, "System message should be preserved")

	assert.Greater(t, len(result), 0, "Should have at least one message")
}

func TestCalculateImportanceScore(t *testing.T) {
	now := time.Now()

	systemMsg := Message{
		Type:       MessageTypeSystem,
		Priority:   10,
		Timestamp:  now,
		LastAccess: now,
	}

	userMsg := Message{
		Type:       MessageTypeUser,
		Priority:   5,
		Timestamp:  now,
		LastAccess: now,
	}

	toolMsg := Message{
		Type:       MessageTypeToolUse,
		Priority:   0,
		Timestamp:  now,
		LastAccess: now,
	}

	allMessages := []Message{systemMsg, userMsg, toolMsg}

	systemScore := calculateImportanceScore(systemMsg, allMessages)
	userScore := calculateImportanceScore(userMsg, allMessages)
	toolScore := calculateImportanceScore(toolMsg, allMessages)

	assert.Greater(t, systemScore, userScore, "System messages should have higher score than user messages")
	assert.Greater(t, userScore, toolScore, "User messages should have higher score than tool messages")
}

func TestTruncationEmptyMessages(t *testing.T) {
	messages := []Message{}
	maxTokens := 100

	result, err := truncateOldest(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 0)

	result, err = truncateLRU(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 0)

	result, err = truncateByType(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 0)

	result, err = truncateByPriority(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 0)

	result, err = truncateIntelligent(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestTruncationNoTruncationNeeded(t *testing.T) {
	messages := []Message{
		{
			Type:       MessageTypeUser,
			Content:    "Short",
			TokenCount: 2,
		},
		{
			Type:       MessageTypeAssistant,
			Content:    "Response",
			TokenCount: 2,
		},
	}
	maxTokens := 100

	result, err := truncateOldest(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	result, err = truncateLRU(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	result, err = truncateByType(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	result, err = truncateByPriority(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	result, err = truncateIntelligent(messages, maxTokens)
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestApplyTruncationStrategySelection(t *testing.T) {
	messages := createTestMessages()
	maxTokens := 20

	strategies := []TruncationStrategy{
		TruncationOldest,
		TruncationLRU,
		TruncationByType,
		TruncationByPriority,
		TruncationIntelligent,
	}

	for _, strategy := range strategies {
		result, err := ApplyTruncation(strategy, messages, maxTokens)
		require.NoError(t, err, "Strategy %s should not error", strategy)

		totalTokens := 0
		for _, msg := range result {
			totalTokens += msg.TokenCount
		}
		assert.LessOrEqual(t, totalTokens, maxTokens, "Strategy %s should respect token limit", strategy)
	}
}

func TestApplyTruncationDefaultStrategy(t *testing.T) {
	messages := createTestMessages()
	maxTokens := 20

	result, err := ApplyTruncation("invalid_strategy", messages, maxTokens)
	require.NoError(t, err)

	totalTokens := 0
	for _, msg := range result {
		totalTokens += msg.TokenCount
	}
	assert.LessOrEqual(t, totalTokens, maxTokens)
}