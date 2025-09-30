package context

import (
	"fmt"
	"sort"
)

type TruncationStrategy string

const (
	TruncationOldest      TruncationStrategy = "oldest"
	TruncationLRU         TruncationStrategy = "lru"
	TruncationByType      TruncationStrategy = "by_type"
	TruncationByPriority  TruncationStrategy = "by_priority"
	TruncationIntelligent TruncationStrategy = "intelligent"
)

func ApplyTruncation(strategy TruncationStrategy, messages []Message, maxTokens int) ([]Message, error) {
	switch strategy {
	case TruncationOldest:
		return truncateOldest(messages, maxTokens)
	case TruncationLRU:
		return truncateLRU(messages, maxTokens)
	case TruncationByType:
		return truncateByType(messages, maxTokens)
	case TruncationByPriority:
		return truncateByPriority(messages, maxTokens)
	case TruncationIntelligent:
		return truncateIntelligent(messages, maxTokens)
	default:
		return truncateOldest(messages, maxTokens)
	}
}

func truncateOldest(messages []Message, maxTokens int) ([]Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.TokenCount
	}

	if totalTokens <= maxTokens {
		return messages, nil
	}

	systemMessages := []Message{}
	otherMessages := []Message{}

	for _, msg := range messages {
		if msg.Type == MessageTypeSystem {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	totalTokens = 0
	for _, msg := range systemMessages {
		totalTokens += msg.TokenCount
	}

	result := make([]Message, 0, len(messages))
	result = append(result, systemMessages...)

	startIdx := 0
	for i := len(otherMessages) - 1; i >= 0; i-- {
		if totalTokens+otherMessages[i].TokenCount > maxTokens {
			break
		}
		totalTokens += otherMessages[i].TokenCount
		startIdx = i
	}

	result = append(result, otherMessages[startIdx:]...)

	return result, nil
}

func truncateLRU(messages []Message, maxTokens int) ([]Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.TokenCount
	}

	if totalTokens <= maxTokens {
		return messages, nil
	}

	systemMessages := []Message{}
	otherMessages := []Message{}

	for _, msg := range messages {
		if msg.Type == MessageTypeSystem {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	sort.Slice(otherMessages, func(i, j int) bool {
		return otherMessages[i].LastAccess.After(otherMessages[j].LastAccess)
	})

	totalTokens = 0
	for _, msg := range systemMessages {
		totalTokens += msg.TokenCount
	}

	result := make([]Message, 0, len(messages))
	result = append(result, systemMessages...)

	for _, msg := range otherMessages {
		if totalTokens+msg.TokenCount > maxTokens {
			break
		}
		totalTokens += msg.TokenCount
		result = append(result, msg)
	}

	sort.Slice(result[len(systemMessages):], func(i, j int) bool {
		return result[len(systemMessages)+i].Timestamp.Before(result[len(systemMessages)+j].Timestamp)
	})

	return result, nil
}

func truncateByType(messages []Message, maxTokens int) ([]Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.TokenCount
	}

	if totalTokens <= maxTokens {
		return messages, nil
	}

	typePriority := map[MessageType]int{
		MessageTypeSystem:     4,
		MessageTypeUser:       3,
		MessageTypeAssistant:  2,
		MessageTypeToolUse:    1,
		MessageTypeToolResult: 1,
	}

	sorted := make([]Message, len(messages))
	copy(sorted, messages)

	sort.Slice(sorted, func(i, j int) bool {
		priorityI := typePriority[sorted[i].Type]
		priorityJ := typePriority[sorted[j].Type]

		if priorityI != priorityJ {
			return priorityI > priorityJ
		}

		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})

	totalTokens = 0
	result := make([]Message, 0, len(messages))

	for _, msg := range sorted {
		if totalTokens+msg.TokenCount > maxTokens {
			break
		}
		totalTokens += msg.TokenCount
		result = append(result, msg)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result, nil
}

func truncateByPriority(messages []Message, maxTokens int) ([]Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.TokenCount
	}

	if totalTokens <= maxTokens {
		return messages, nil
	}

	sorted := make([]Message, len(messages))
	copy(sorted, messages)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})

	totalTokens = 0
	result := make([]Message, 0, len(messages))

	for _, msg := range sorted {
		if totalTokens+msg.TokenCount > maxTokens {
			break
		}
		totalTokens += msg.TokenCount
		result = append(result, msg)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result, nil
}

func truncateIntelligent(messages []Message, maxTokens int) ([]Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.TokenCount
	}

	if totalTokens <= maxTokens {
		return messages, nil
	}

	scored := make([]struct {
		message Message
		score   float64
	}, len(messages))

	for i, msg := range messages {
		score := calculateImportanceScore(msg, messages)
		scored[i] = struct {
			message Message
			score   float64
		}{message: msg, score: score}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	totalTokens = 0
	result := make([]Message, 0, len(messages))

	for _, item := range scored {
		if totalTokens+item.message.TokenCount > maxTokens {
			break
		}
		totalTokens += item.message.TokenCount
		result = append(result, item.message)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	if len(result) == 0 {
		return nil, fmt.Errorf("cannot fit any messages within token limit")
	}

	return result, nil
}

func calculateImportanceScore(msg Message, allMessages []Message) float64 {
	score := 0.0

	switch msg.Type {
	case MessageTypeSystem:
		score += 100.0
	case MessageTypeUser:
		score += 50.0
	case MessageTypeAssistant:
		score += 30.0
	case MessageTypeToolUse:
		score += 20.0
	case MessageTypeToolResult:
		score += 25.0
	}

	score += float64(msg.Priority) * 10.0

	recency := float64(msg.Timestamp.Unix()) / 1000000.0
	score += recency

	lruScore := float64(msg.LastAccess.Unix()) / 1000000.0
	score += lruScore * 0.5

	return score
}