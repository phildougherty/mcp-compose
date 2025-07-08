package dashboard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type ActivityStorage struct {
	db *sql.DB
}

type StoredActivity struct {
	ID         int64                  `json:"id"`
	ActivityID string                 `json:"activity_id"`
	Timestamp  time.Time              `json:"timestamp"`
	Level      string                 `json:"level"`
	Type       string                 `json:"type"`
	Server     string                 `json:"server"`
	Client     string                 `json:"client"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details"`
	CreatedAt  time.Time              `json:"created_at"`
}

func NewActivityStorage(dbURL string) (*ActivityStorage, error) {
	log.Printf("[ACTIVITY] Initializing activity storage...")

	// Parse the database URL to extract database name
	dbName, err := extractDatabaseName(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	log.Printf("[ACTIVITY] Target database: %s", dbName)

	// Create database if it doesn't exist
	if err := createDatabaseIfNotExists(dbURL, dbName); err != nil {
		return nil, fmt.Errorf("failed to ensure database exists: %w", err)
	}

	// Now connect to the target database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	storage := &ActivityStorage{db: db}

	// Create tables if they don't exist
	if err := storage.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	log.Printf("[ACTIVITY] Activity storage initialized successfully")
	return storage, nil
}

// extractDatabaseName extracts the database name from a PostgreSQL connection URL
func extractDatabaseName(dbURL string) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", err
	}

	// Database name is the path without the leading slash
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return "", fmt.Errorf("no database name found in URL")
	}

	// Remove any query parameters from the database name
	if idx := strings.Index(dbName, "?"); idx >= 0 {
		dbName = dbName[:idx]
	}

	return dbName, nil
}

// createDatabaseIfNotExists ensures the target database exists
func createDatabaseIfNotExists(dbURL, targetDB string) error {
	log.Printf("[ACTIVITY] Checking if database '%s' exists...", targetDB)

	// Parse the original URL
	u, err := url.Parse(dbURL)
	if err != nil {
		return err
	}

	// Create a connection URL to the 'postgres' system database
	systemURL := *u
	systemURL.Path = "/postgres"
	systemConnection := systemURL.String()

	// Connect to the PostgreSQL system database
	db, err := sql.Open("postgres", systemConnection)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres system database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Printf("Warning: failed to close database connection: %v\n", err)
		}
	}()

	// Check if target database exists
	var exists bool
	checkQuery := "SELECT EXISTS(SELECT datname FROM pg_database WHERE datname = $1)"
	err = db.QueryRow(checkQuery, targetDB).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		log.Printf("[ACTIVITY] Database '%s' already exists", targetDB)
		return nil
	}

	// Create the database
	log.Printf("[ACTIVITY] Creating database '%s'...", targetDB)
	createQuery := fmt.Sprintf("CREATE DATABASE %s", targetDB)
	_, err = db.Exec(createQuery)
	if err != nil {
		return fmt.Errorf("failed to create database '%s': %w", targetDB, err)
	}

	log.Printf("[ACTIVITY] Database '%s' created successfully", targetDB)
	return nil
}

func (s *ActivityStorage) initTables() error {
	log.Printf("[ACTIVITY] Initializing activity tables...")

	query := `
    CREATE TABLE IF NOT EXISTS activity_events (
        id BIGSERIAL PRIMARY KEY,
        activity_id VARCHAR(255) NOT NULL,
        timestamp TIMESTAMPTZ NOT NULL,
        level VARCHAR(50) NOT NULL,
        type VARCHAR(100) NOT NULL,
        server VARCHAR(255),
        client VARCHAR(255),
        message TEXT NOT NULL,
        details JSONB,
        created_at TIMESTAMPTZ DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_activity_events_timestamp ON activity_events(timestamp);
    CREATE INDEX IF NOT EXISTS idx_activity_events_level ON activity_events(level);
    CREATE INDEX IF NOT EXISTS idx_activity_events_type ON activity_events(type);
    CREATE INDEX IF NOT EXISTS idx_activity_events_server ON activity_events(server);
    CREATE INDEX IF NOT EXISTS idx_activity_events_created_at ON activity_events(created_at);
    `

	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Printf("[ACTIVITY] Activity tables initialized successfully")
	return nil
}

func (s *ActivityStorage) StoreActivity(activity ActivityMessage) error {
	detailsJSON, err := json.Marshal(activity.Details)
	if err != nil {
		return fmt.Errorf("failed to marshal details: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339Nano, activity.Timestamp)
	if err != nil {
		timestamp = time.Now()
	}

	query := `
    INSERT INTO activity_events (activity_id, timestamp, level, type, server, client, message, details)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `

	_, err = s.db.Exec(query, activity.ID, timestamp, activity.Level, activity.Type,
		activity.Server, activity.Client, activity.Message, string(detailsJSON))

	if err != nil {
		return fmt.Errorf("failed to store activity: %w", err)
	}

	return nil
}

func (s *ActivityStorage) GetRecentActivities(limit int, since *time.Time) ([]StoredActivity, error) {
	query := `
    SELECT id, activity_id, timestamp, level, type, 
           COALESCE(server, '') as server, 
           COALESCE(client, '') as client, 
           message, COALESCE(details, '{}') as details, created_at
    FROM activity_events
    `
	args := []interface{}{}

	if since != nil {
		query += " WHERE timestamp >= $1"
		args = append(args, *since)
	}

	query += " ORDER BY timestamp DESC"

	if limit > 0 {
		if len(args) > 0 {
			query += " LIMIT $2"
		} else {
			query += " LIMIT $1"
		}
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			fmt.Printf("Warning: failed to close rows: %v\n", err)
		}
	}()

	var activities []StoredActivity
	for rows.Next() {
		var activity StoredActivity
		var detailsJSON string

		err := rows.Scan(&activity.ID, &activity.ActivityID, &activity.Timestamp,
			&activity.Level, &activity.Type, &activity.Server, &activity.Client,
			&activity.Message, &detailsJSON, &activity.CreatedAt)
		if err != nil {
			return nil, err
		}

		// Parse details JSON
		if detailsJSON != "" && detailsJSON != "{}" {
			if err := json.Unmarshal([]byte(detailsJSON), &activity.Details); err != nil {
				activity.Details = make(map[string]interface{})
			}
		} else {
			activity.Details = make(map[string]interface{})
		}

		activities = append(activities, activity)
	}

	return activities, rows.Err()
}

func (s *ActivityStorage) CleanupOldActivities(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	query := "DELETE FROM activity_events WHERE created_at < $1"
	result, err := s.db.Exec(query, cutoff)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("[ACTIVITY] Cleaned up %d old activity records", rowsAffected)
	}

	return nil
}

func (s *ActivityStorage) GetActivityStats() (map[string]interface{}, error) {
	query := `
    SELECT 
        COUNT(*) as total,
        COUNT(CASE WHEN level = 'ERROR' THEN 1 END) as errors,
        COUNT(CASE WHEN level = 'WARN' THEN 1 END) as warnings,
        COUNT(CASE WHEN level = 'INFO' THEN 1 END) as info,
        COUNT(CASE WHEN type = 'request' THEN 1 END) as requests,
        COUNT(CASE WHEN 
            type ILIKE '%tool%' OR 
            message ILIKE '%tool%' OR 
            details ? 'toolCall' OR 
            details ? 'tool_call' OR
            details::text ILIKE '%tool%'
        THEN 1 END) as tool_calls,
        COUNT(CASE WHEN created_at >= NOW() - INTERVAL '24 hours' THEN 1 END) as last_24h,
        COUNT(CASE WHEN created_at >= NOW() - INTERVAL '1 hour' THEN 1 END) as last_1h
    FROM activity_events
    WHERE created_at >= CURRENT_DATE  -- Today's activities only
    `

	var stats struct {
		Total     int `json:"total"`
		Errors    int `json:"errors"`
		Warnings  int `json:"warnings"`
		Info      int `json:"info"`
		Requests  int `json:"requests"`
		ToolCalls int `json:"tool_calls"`
		Last24h   int `json:"last_24h"`
		Last1h    int `json:"last_1h"`
	}

	err := s.db.QueryRow(query).Scan(&stats.Total, &stats.Errors, &stats.Warnings,
		&stats.Info, &stats.Requests, &stats.ToolCalls, &stats.Last24h, &stats.Last1h)
	if err != nil {
		return nil, err
	}

	// Return in the format the frontend expects
	return map[string]interface{}{
		"totalToday":     stats.Total,
		"requestsToday":  stats.Requests,
		"errorsToday":    stats.Errors,
		"toolCallsToday": stats.ToolCalls,
		"warningsToday":  stats.Warnings,
		"infoToday":      stats.Info,
		"last24h":        stats.Last24h,
		"last1h":         stats.Last1h,

		// Keep old field names for compatibility
		"total":           stats.Total,
		"errors":          stats.Errors,
		"warnings":        stats.Warnings,
		"info":            stats.Info,
		"tasks":           0,
		"task_management": 0,
	}, nil
}

func (s *ActivityStorage) Close() error {
	if s.db != nil {
		log.Printf("[ACTIVITY] Closing activity storage connection")
		return s.db.Close()
	}
	return nil
}
