package context_test

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/phildougherty/mcp-compose/internal/context"
)

func ExampleManager_basic() {
	cfg := context.Config{
		MaxTokens:          1000,
		TruncationStrategy: context.TruncationOldest,
		PersistenceEnabled: false,
	}

	manager, err := context.NewManager(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer manager.Close()

	conversationID := "example-conv"

	msg := context.Message{
		Type:    context.MessageTypeUser,
		Content: "Hello, world!",
	}

	err = manager.AddMessage(conversationID, msg)
	if err != nil {
		log.Fatal(err)
	}

	messages, err := manager.GetMessages(conversationID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Messages: %d\n", len(messages))
}

func ExampleManager_withPersistence() {
	dbPath := "./example-context.db"
	defer os.Remove(dbPath)

	cfg := context.Config{
		MaxTokens:          32000,
		TruncationStrategy: context.TruncationIntelligent,
		PersistenceEnabled: true,
		DatabasePath:       dbPath,
		ContextTTL:         24 * time.Hour,
	}

	manager, err := context.NewManager(cfg)
	if err != nil {
		log.Fatal(err)
	}

	conversationID := "persistent-conv"

	systemMsg := context.Message{
		Type:     context.MessageTypeSystem,
		Content:  "You are a helpful assistant.",
		Priority: 10,
	}

	userMsg := context.Message{
		Type:    context.MessageTypeUser,
		Content: "What is AI?",
	}

	manager.AddMessage(conversationID, systemMsg)
	manager.AddMessage(conversationID, userMsg)

	manager.Close()

	manager2, err := context.NewManager(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer manager2.Close()

	messages, err := manager2.GetMessages(conversationID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Restored messages: %d\n", len(messages))
}

func ExampleManager_tokenCounting() {
	cfg := context.Config{
		MaxTokens: 100,
		Model:     "gpt-4",
	}

	manager, err := context.NewManager(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer manager.Close()

	text := "The quick brown fox jumps over the lazy dog."
	tokens := manager.CountTokens(text)

	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Tokens: %d\n", tokens)
}

func ExampleManager_truncation() {
	cfg := context.Config{
		MaxTokens:          50,
		TruncationStrategy: context.TruncationIntelligent,
		PersistenceEnabled: false,
	}

	manager, err := context.NewManager(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer manager.Close()

	conversationID := "truncation-example"

	messages := []context.Message{
		{
			Type:     context.MessageTypeSystem,
			Content:  "You are helpful.",
			Priority: 10,
		},
		{
			Type:    context.MessageTypeUser,
			Content: "Tell me a long story about a knight who goes on many adventures across the land.",
		},
		{
			Type:    context.MessageTypeAssistant,
			Content: "Once upon a time, there was a brave knight...",
		},
		{
			Type:    context.MessageTypeUser,
			Content: "Continue please.",
		},
	}

	for _, msg := range messages {
		manager.AddMessage(conversationID, msg)
	}

	retrieved, _ := manager.GetMessages(conversationID)
	totalTokens := manager.GetTotalTokens(conversationID)

	fmt.Printf("Messages after truncation: %d\n", len(retrieved))
	fmt.Printf("Total tokens: %d (max: %d)\n", totalTokens, cfg.MaxTokens)
}

func ExampleManager_multipleStrategies() {
	strategies := []context.TruncationStrategy{
		context.TruncationOldest,
		context.TruncationLRU,
		context.TruncationByType,
		context.TruncationByPriority,
		context.TruncationIntelligent,
	}

	for _, strategy := range strategies {
		cfg := context.Config{
			MaxTokens:          50,
			TruncationStrategy: strategy,
			PersistenceEnabled: false,
		}

		manager, err := context.NewManager(cfg)
		if err != nil {
			log.Fatal(err)
		}

		conversationID := fmt.Sprintf("conv-%s", strategy)

		manager.AddMessage(conversationID, context.Message{
			Type:    context.MessageTypeSystem,
			Content: "System message.",
		})
		manager.AddMessage(conversationID, context.Message{
			Type:    context.MessageTypeUser,
			Content: "Long user message that takes many tokens to represent.",
		})
		manager.AddMessage(conversationID, context.Message{
			Type:    context.MessageTypeAssistant,
			Content: "Assistant response.",
		})

		messages, _ := manager.GetMessages(conversationID)
		fmt.Printf("Strategy %s: %d messages\n", strategy, len(messages))

		manager.Close()
	}
}

func ExampleManager_listConversations() {
	cfg := context.Config{
		MaxTokens:          1000,
		PersistenceEnabled: false,
	}

	manager, err := context.NewManager(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer manager.Close()

	convIDs := []string{"conv1", "conv2", "conv3"}
	for _, id := range convIDs {
		manager.AddMessage(id, context.Message{
			Type:    context.MessageTypeUser,
			Content: "Hello!",
		})
	}

	conversations, err := manager.ListConversations()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Active conversations: %d\n", len(conversations))
}