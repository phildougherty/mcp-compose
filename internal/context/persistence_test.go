package context

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPersistenceManager(t *testing.T) {
	dbPath := "./test_persistence.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	require.NotNil(t, pm)
	defer pm.Close()
}

func TestNewPersistenceManagerDefaultPath(t *testing.T) {
	pm, err := NewPersistenceManager("")
	require.NoError(t, err)
	require.NotNil(t, pm)
	defer pm.Close()
	defer os.Remove("./mcp-compose-context.db")
}

func TestSaveAndLoadMessage(t *testing.T) {
	dbPath := "./test_save_load.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	msg := Message{
		ID:         "msg1",
		Type:       MessageTypeUser,
		Content:    "Test message",
		Timestamp:  time.Now(),
		Priority:   5,
		TokenCount: 10,
		LastAccess: time.Now(),
	}

	err = pm.SaveMessage("conv1", msg)
	require.NoError(t, err)

	messages, err := pm.LoadConversation("conv1")
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, msg.ID, messages[0].ID)
	assert.Equal(t, msg.Type, messages[0].Type)
	assert.Equal(t, msg.Content, messages[0].Content)
	assert.Equal(t, msg.Priority, messages[0].Priority)
	assert.Equal(t, msg.TokenCount, messages[0].TokenCount)
}

func TestSaveMultipleMessages(t *testing.T) {
	dbPath := "./test_multiple.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	messages := []Message{
		{
			ID:         "msg1",
			Type:       MessageTypeSystem,
			Content:    "System message",
			Timestamp:  time.Now(),
			TokenCount: 5,
			LastAccess: time.Now(),
		},
		{
			ID:         "msg2",
			Type:       MessageTypeUser,
			Content:    "User message",
			Timestamp:  time.Now(),
			TokenCount: 5,
			LastAccess: time.Now(),
		},
		{
			ID:         "msg3",
			Type:       MessageTypeAssistant,
			Content:    "Assistant message",
			Timestamp:  time.Now(),
			TokenCount: 5,
			LastAccess: time.Now(),
		},
	}

	for _, msg := range messages {
		err := pm.SaveMessage("conv1", msg)
		require.NoError(t, err)
	}

	loaded, err := pm.LoadConversation("conv1")
	require.NoError(t, err)
	require.Len(t, loaded, 3)

	assert.Equal(t, MessageTypeSystem, loaded[0].Type)
	assert.Equal(t, MessageTypeUser, loaded[1].Type)
	assert.Equal(t, MessageTypeAssistant, loaded[2].Type)
}

func TestDeleteConversation(t *testing.T) {
	dbPath := "./test_delete.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	msg := Message{
		ID:         "msg1",
		Type:       MessageTypeUser,
		Content:    "Test message",
		Timestamp:  time.Now(),
		TokenCount: 10,
		LastAccess: time.Now(),
	}

	err = pm.SaveMessage("conv1", msg)
	require.NoError(t, err)

	messages, err := pm.LoadConversation("conv1")
	require.NoError(t, err)
	require.Len(t, messages, 1)

	err = pm.DeleteConversation("conv1")
	require.NoError(t, err)

	messages, err = pm.LoadConversation("conv1")
	require.NoError(t, err)
	require.Len(t, messages, 0)
}

func TestPersistenceListConversations(t *testing.T) {
	dbPath := "./test_list.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	conversationIDs := []string{"conv1", "conv2", "conv3"}

	for _, id := range conversationIDs {
		msg := Message{
			ID:         "msg_" + id,
			Type:       MessageTypeUser,
			Content:    "Test message",
			Timestamp:  time.Now(),
			TokenCount: 10,
			LastAccess: time.Now(),
		}
		err := pm.SaveMessage(id, msg)
		require.NoError(t, err)
	}

	ids, err := pm.ListConversations()
	require.NoError(t, err)
	require.Len(t, ids, 3)

	for _, id := range conversationIDs {
		assert.Contains(t, ids, id)
	}
}

func TestDeleteOldMessages(t *testing.T) {
	dbPath := "./test_delete_old.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	oldMsg := Message{
		ID:         "old_msg",
		Type:       MessageTypeUser,
		Content:    "Old message",
		Timestamp:  time.Now().Add(-48 * time.Hour),
		TokenCount: 10,
		LastAccess: time.Now().Add(-48 * time.Hour),
	}

	newMsg := Message{
		ID:         "new_msg",
		Type:       MessageTypeUser,
		Content:    "New message",
		Timestamp:  time.Now(),
		TokenCount: 10,
		LastAccess: time.Now(),
	}

	err = pm.SaveMessage("conv1", oldMsg)
	require.NoError(t, err)

	err = pm.SaveMessage("conv1", newMsg)
	require.NoError(t, err)

	cutoff := time.Now().Add(-24 * time.Hour)
	err = pm.DeleteOldMessages(cutoff)
	require.NoError(t, err)

	messages, err := pm.LoadConversation("conv1")
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, "new_msg", messages[0].ID)
}

func TestGetConversationStats(t *testing.T) {
	dbPath := "./test_stats.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	messages := []Message{
		{
			ID:         "msg1",
			Type:       MessageTypeUser,
			Content:    "Message 1",
			Timestamp:  time.Now().Add(-10 * time.Minute),
			TokenCount: 5,
			LastAccess: time.Now(),
		},
		{
			ID:         "msg2",
			Type:       MessageTypeUser,
			Content:    "Message 2",
			Timestamp:  time.Now(),
			TokenCount: 7,
			LastAccess: time.Now(),
		},
	}

	for _, msg := range messages {
		err := pm.SaveMessage("conv1", msg)
		require.NoError(t, err)
	}

	stats, err := pm.GetConversationStats("conv1")
	require.NoError(t, err)

	assert.Equal(t, 2, stats["message_count"])
	assert.Equal(t, 12, stats["total_tokens"])
	assert.NotNil(t, stats["oldest_message"])
	assert.NotNil(t, stats["newest_message"])
}

func TestGetMessagesPaginated(t *testing.T) {
	dbPath := "./test_paginated.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	for i := 0; i < 10; i++ {
		msg := Message{
			ID:         "msg" + string(rune('0'+i)),
			Type:       MessageTypeUser,
			Content:    "Message",
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
			TokenCount: 5,
			LastAccess: time.Now(),
		}
		err := pm.SaveMessage("conv1", msg)
		require.NoError(t, err)
	}

	page1, err := pm.GetMessagesPaginated("conv1", 0, 5)
	require.NoError(t, err)
	require.Len(t, page1, 5)

	page2, err := pm.GetMessagesPaginated("conv1", 5, 5)
	require.NoError(t, err)
	require.Len(t, page2, 5)

	assert.NotEqual(t, page1[0].ID, page2[0].ID)
}

func TestExportAndImportConversation(t *testing.T) {
	dbPath := "./test_export_import.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	messages := []Message{
		{
			ID:         "msg1",
			Type:       MessageTypeSystem,
			Content:    "System message",
			Timestamp:  time.Now(),
			Priority:   10,
			TokenCount: 5,
			LastAccess: time.Now(),
		},
		{
			ID:         "msg2",
			Type:       MessageTypeUser,
			Content:    "User message",
			Timestamp:  time.Now(),
			Priority:   5,
			TokenCount: 5,
			LastAccess: time.Now(),
		},
	}

	for _, msg := range messages {
		err := pm.SaveMessage("conv1", msg)
		require.NoError(t, err)
	}

	data, err := pm.ExportConversation("conv1")
	require.NoError(t, err)
	require.NotEmpty(t, data)

	err = pm.DeleteConversation("conv1")
	require.NoError(t, err)

	err = pm.ImportConversation("conv1", data)
	require.NoError(t, err)

	loaded, err := pm.LoadConversation("conv1")
	require.NoError(t, err)
	require.Len(t, loaded, 2)

	assert.Equal(t, MessageTypeSystem, loaded[0].Type)
	assert.Equal(t, MessageTypeUser, loaded[1].Type)
}

func TestMessageOrdering(t *testing.T) {
	dbPath := "./test_ordering.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	now := time.Now()
	messages := []Message{
		{
			ID:         "msg3",
			Type:       MessageTypeUser,
			Content:    "Third",
			Timestamp:  now.Add(2 * time.Minute),
			TokenCount: 5,
			LastAccess: now,
		},
		{
			ID:         "msg1",
			Type:       MessageTypeUser,
			Content:    "First",
			Timestamp:  now,
			TokenCount: 5,
			LastAccess: now,
		},
		{
			ID:         "msg2",
			Type:       MessageTypeUser,
			Content:    "Second",
			Timestamp:  now.Add(1 * time.Minute),
			TokenCount: 5,
			LastAccess: now,
		},
	}

	for _, msg := range messages {
		err := pm.SaveMessage("conv1", msg)
		require.NoError(t, err)
	}

	loaded, err := pm.LoadConversation("conv1")
	require.NoError(t, err)
	require.Len(t, loaded, 3)

	assert.Equal(t, "msg1", loaded[0].ID)
	assert.Equal(t, "msg2", loaded[1].ID)
	assert.Equal(t, "msg3", loaded[2].ID)
}

func TestConcurrentSave(t *testing.T) {
	dbPath := "./test_concurrent.db"
	defer os.Remove(dbPath)

	pm, err := NewPersistenceManager(dbPath)
	require.NoError(t, err)
	defer pm.Close()

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			msg := Message{
				ID:         "msg" + string(rune('0'+id)),
				Type:       MessageTypeUser,
				Content:    "Concurrent message",
				Timestamp:  time.Now(),
				TokenCount: 5,
				LastAccess: time.Now(),
			}
			err := pm.SaveMessage("conv1", msg)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	messages, err := pm.LoadConversation("conv1")
	require.NoError(t, err)
	assert.Len(t, messages, 10)
}