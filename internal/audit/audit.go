package audit

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/constants"
	"mcpcompose/internal/logging"
)

const (
	// Default audit retention period in days
	DefaultAuditRetentionDays = 7
	// Default audit statistics timeout in seconds
	DefaultAuditStatsTimeout = 5
	// Percentage multiplier for success rate calculation
	PercentageMultiplier = 100
)

var (
	// ErrAuditShutdownTimeout is returned when audit logger shutdown times out.
	ErrAuditShutdownTimeout = errors.New("audit logger shutdown timeout")
)

type AuditLogger struct {
	enabled    bool
	storage    string
	maxEntries int
	maxAge     time.Duration
	events     map[string]bool
	entries    []AuditEntry
	mu         sync.RWMutex
	logger     *logging.Logger
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

type AuditEntry struct {
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

func NewAuditLogger(auditConfig *config.AuditConfig, logger *logging.Logger) *AuditLogger {
	maxAge, _ := time.ParseDuration(auditConfig.Retention.MaxAge)
	if maxAge == 0 {
		maxAge = DefaultAuditRetentionDays * constants.HoursInDay * time.Hour // Default 7 days
	}

	events := make(map[string]bool)
	for _, event := range auditConfig.Events {
		events[event] = true
	}

	al := &AuditLogger{
		enabled:    auditConfig.Enabled,
		storage:    auditConfig.Storage,
		maxEntries: auditConfig.Retention.MaxEntries,
		maxAge:     maxAge,
		events:     events,
		entries:    make([]AuditEntry, 0),
		logger:     logger,
		stopCh:     make(chan struct{}),
	}

	// Start cleanup routine with proper resource management
	al.wg.Add(1)
	go al.cleanupOldEntries()


	return al
}

func (al *AuditLogger) Log(event, userID, clientID, ip, userAgent string, success bool, details map[string]interface{}, err error) {
	if !al.enabled {

		return
	}

	// Check if this event type should be logged
	if !al.events[event] {

		return
	}

	entry := AuditEntry{
		ID:        generateAuditID(),
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

	al.storeEntry(&entry)

	// Also log to standard logger
	level := "info"
	if !success {
		level = "warn"
	}

	// Fix: Use the correct method name
	if level == "info" {
		al.logger.Info("AUDIT: %s - User: %s, Client: %s, Success: %v", event, userID, clientID, success)
	} else {
		al.logger.Warning("AUDIT: %s - User: %s, Client: %s, Success: %v", event, userID, clientID, success)
	}
}

func (al *AuditLogger) storeEntry(entry *AuditEntry) {
	al.mu.Lock()
	defer al.mu.Unlock()

	switch al.storage {
	case "memory":
		al.entries = append(al.entries, *entry)
		// Trim if over max entries
		if len(al.entries) > al.maxEntries {
			al.entries = al.entries[len(al.entries)-al.maxEntries:]
		}
	case "file":
		// File storage not implemented - using memory fallback
		al.logger.Warning("File storage not implemented, using memory storage as fallback")
		al.entries = append(al.entries, *entry)
		if len(al.entries) > al.maxEntries {
			al.entries = al.entries[len(al.entries)-al.maxEntries:]
		}
	case "database":
		// Database storage not implemented - using memory fallback
		al.logger.Warning("Database storage not implemented, using memory storage as fallback")
		al.entries = append(al.entries, *entry)
		if len(al.entries) > al.maxEntries {
			al.entries = al.entries[len(al.entries)-al.maxEntries:]
		}
	}
}

func (al *AuditLogger) GetEntries(limit int, offset int, filter *AuditFilter) ([]AuditEntry, int, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var filtered []AuditEntry
	for _, entry := range al.entries {
		if al.matchesFilter(entry, filter) {
			filtered = append(filtered, entry)
		}
	}

	total := len(filtered)

	// Apply pagination
	start := offset
	if start > len(filtered) {
		start = len(filtered)
	}

	end := start + limit
	if end > len(filtered) {
		end = len(filtered)
	}


	return filtered[start:end], total, nil
}

type AuditFilter struct {
	Event     string    `json:"event,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	ClientID  string    `json:"client_id,omitempty"`
	Success   *bool     `json:"success,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

func (al *AuditLogger) matchesFilter(entry AuditEntry, filter *AuditFilter) bool {

	return al.matchesEvent(entry, filter) &&
		al.matchesUser(entry, filter) &&
		al.matchesClient(entry, filter) &&
		al.matchesSuccess(entry, filter) &&
		al.matchesTimeRange(entry, filter)
}

func (al *AuditLogger) matchesEvent(entry AuditEntry, filter *AuditFilter) bool {

	return filter.Event == "" || entry.Event == filter.Event
}

func (al *AuditLogger) matchesUser(entry AuditEntry, filter *AuditFilter) bool {

	return filter.UserID == "" || entry.UserID == filter.UserID
}

func (al *AuditLogger) matchesClient(entry AuditEntry, filter *AuditFilter) bool {

	return filter.ClientID == "" || entry.ClientID == filter.ClientID
}

func (al *AuditLogger) matchesSuccess(entry AuditEntry, filter *AuditFilter) bool {

	return filter.Success == nil || entry.Success == *filter.Success
}

func (al *AuditLogger) matchesTimeRange(entry AuditEntry, filter *AuditFilter) bool {
	if !filter.StartTime.IsZero() && entry.Timestamp.Before(filter.StartTime) {

		return false
	}
	if !filter.EndTime.IsZero() && entry.Timestamp.After(filter.EndTime) {

		return false
	}


	return true
}

func (al *AuditLogger) cleanupOldEntries() {
	defer al.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-al.stopCh:
			al.logger.Debug("Audit logger cleanup goroutine stopping")


			return
		case <-ticker.C:
			al.mu.Lock()
			cutoff := time.Now().Add(-al.maxAge)
			var kept []AuditEntry

			for _, entry := range al.entries {
				if entry.Timestamp.After(cutoff) {
					kept = append(kept, entry)
				}
			}

			if len(kept) != len(al.entries) {
				al.logger.Debug("Cleaned up %d old audit entries", len(al.entries)-len(kept))
			}
			al.entries = kept
			al.mu.Unlock()
		}
	}
}

// Shutdown gracefully stops the audit logger
func (al *AuditLogger) Shutdown() error {
	if al.stopCh != nil {
		close(al.stopCh)
	}

	// Wait for cleanup goroutine to finish with timeout
	done := make(chan struct{})
	go func() {
		al.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		al.logger.Debug("Audit logger shutdown completed")


		return nil
	case <-time.After(DefaultAuditStatsTimeout * time.Second):
		al.logger.Warning("Audit logger shutdown timeout")


		return ErrAuditShutdownTimeout
	}
}

func (al *AuditLogger) GetStats() AuditStats {
	al.mu.RLock()
	defer al.mu.RUnlock()

	stats := AuditStats{
		TotalEntries: len(al.entries),
		EventCounts:  make(map[string]int),
		SuccessRate:  0,
	}

	successCount := 0
	for _, entry := range al.entries {
		stats.EventCounts[entry.Event]++
		if entry.Success {
			successCount++
		}
	}

	if len(al.entries) > 0 {
		stats.SuccessRate = float64(successCount) / float64(len(al.entries)) * PercentageMultiplier
	}


	return stats
}

type AuditStats struct {
	TotalEntries int            `json:"total_entries"`
	EventCounts  map[string]int `json:"event_counts"`
	SuccessRate  float64        `json:"success_rate"`
}

func generateAuditID() string {

	return fmt.Sprintf("audit_%d", time.Now().UnixNano())
}

// Helper methods for common audit events
func (al *AuditLogger) LogOAuthTokenIssued(userID, clientID, ip, userAgent string, tokenType string, success bool, err error) {
	details := map[string]interface{}{
		"token_type": tokenType,
	}
	al.Log("oauth.token.issued", userID, clientID, ip, userAgent, success, details, err)
}

func (al *AuditLogger) LogOAuthTokenRevoked(userID, clientID, ip, userAgent string, tokenType string, success bool, err error) {
	details := map[string]interface{}{
		"token_type": tokenType,
	}
	al.Log("oauth.token.revoked", userID, clientID, ip, userAgent, success, details, err)
}

func (al *AuditLogger) LogServerAccess(userID, clientID, ip, userAgent string, serverName, scope string, success bool, err error) {
	details := map[string]interface{}{
		"server_name": serverName,
		"scope":       scope,
	}
	event := "server.access.granted"
	if !success {
		event = "server.access.denied"
	}
	al.Log(event, userID, clientID, ip, userAgent, success, details, err)
}

func (al *AuditLogger) LogUserLogin(userID, ip, userAgent string, success bool, err error) {
	al.Log("oauth.user.login", userID, "", ip, userAgent, success, nil, err)
}
