// internal/audit/logger.go
package audit

import (
	"fmt"
	"sync"
	"time"

	"mcpcompose/internal/constants"
	"mcpcompose/internal/logging"
)

type Logger struct {
	enabled    bool
	maxEntries int
	maxAge     time.Duration
	events     map[string]bool
	entries    []Entry
	mu         sync.RWMutex
	logger     *logging.Logger
}

type Entry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Event     string                 `json:"event"`
	UserID    string                 `json:"user_id,omitempty"`
	ClientID  string                 `json:"client_id,omitempty"`
	IP        string                 `json:"ip_address,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
}

func NewLogger(maxEntries int, maxAge string, events []string, logger *logging.Logger) *Logger {
	maxAgeDuration, _ := time.ParseDuration(maxAge)
	if maxAgeDuration == 0 {
		maxAgeDuration = DefaultAuditRetentionDays * constants.HoursInDay * time.Hour
	}

	eventMap := make(map[string]bool)
	for _, event := range events {
		eventMap[event] = true
	}

	return &Logger{
		enabled:    true,
		maxEntries: maxEntries,
		maxAge:     maxAgeDuration,
		events:     eventMap,
		entries:    make([]Entry, 0),
		logger:     logger,
	}
}

func (l *Logger) Log(event string, userID, clientID, ip, userAgent string, success bool, details map[string]interface{}, err error) {
	if !l.enabled || !l.events[event] {

		return
	}

	entry := Entry{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Event:     event,
		UserID:    userID,
		ClientID:  clientID,
		IP:        ip,
		UserAgent: userAgent,
		Success:   success,
		Details:   details,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}
	l.mu.Unlock()

	l.logger.Info("AUDIT: %s - User: %s, Client: %s, Success: %v", event, userID, clientID, success)
}

func (l *Logger) GetEntries(limit, offset int) ([]Entry, int) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	total := len(l.entries)
	start := offset
	if start > total {
		start = total
	}

	end := start + limit
	if end > total {
		end = total
	}

	return l.entries[start:end], total
}
