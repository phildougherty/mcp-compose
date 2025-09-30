package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phildougherty/mcp-compose/internal/constants"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

const (
	defaultBatchSize       = 100
	defaultBatchTimeout    = 5 * time.Second
	defaultCleanupInterval = 24 * time.Hour
	defaultRetentionPeriod = 90 * 24 * time.Hour
	maxPoolSize            = 10
	minPoolSize            = 2
	maxConnLifetime        = 1 * time.Hour
	maxConnIdleTime        = 30 * time.Minute
	healthCheckPeriod      = 1 * time.Minute
)

type PostgresStorageConfig struct {
	DatabaseURL     string
	BatchSize       int
	BatchTimeout    time.Duration
	RetentionPeriod time.Duration
	Enabled         bool
}

type PostgresStorage struct {
	config    PostgresStorageConfig
	logger    *logging.Logger
	pool      *pgxpool.Pool
	batchCh   chan *AuditEntry
	mu        sync.Mutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewPostgresStorage(config PostgresStorageConfig, logger *logging.Logger) (*PostgresStorage, error) {
	if !config.Enabled {

		return nil, nil
	}

	if config.DatabaseURL == "" {

		return nil, fmt.Errorf("database URL is required")
	}

	if config.BatchSize <= 0 {
		config.BatchSize = defaultBatchSize
	}
	if config.BatchTimeout <= 0 {
		config.BatchTimeout = defaultBatchTimeout
	}
	if config.RetentionPeriod <= 0 {
		config.RetentionPeriod = defaultRetentionPeriod
	}

	ctx, cancel := context.WithCancel(context.Background())

	poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
	if err != nil {
		cancel()

		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	poolConfig.MaxConns = int32(maxPoolSize)
	poolConfig.MinConns = int32(minPoolSize)
	poolConfig.MaxConnLifetime = maxConnLifetime
	poolConfig.MaxConnIdleTime = maxConnIdleTime
	poolConfig.HealthCheckPeriod = healthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		cancel()

		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		cancel()
		pool.Close()

		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	ps := &PostgresStorage{
		config:  config,
		logger:  logger,
		pool:    pool,
		batchCh: make(chan *AuditEntry, config.BatchSize*2),
		stopCh:  make(chan struct{}),
		ctx:     ctx,
		cancel:  cancel,
	}

	if err := ps.createSchema(ctx); err != nil {
		cancel()
		pool.Close()

		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	ps.wg.Add(2)
	go ps.batchWorker()
	go ps.cleanupWorker()

	logger.Info("PostgreSQL audit storage initialized")

	return ps, nil
}

func (ps *PostgresStorage) createSchema(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS audit_events (
			id BIGSERIAL PRIMARY KEY,
			audit_id VARCHAR(255) NOT NULL UNIQUE,
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
			event VARCHAR(255) NOT NULL,
			user_id VARCHAR(255),
			client_id VARCHAR(255),
			ip_address VARCHAR(255),
			user_agent TEXT,
			details JSONB,
			success BOOLEAN NOT NULL,
			error TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_audit_events_event ON audit_events(event);
		CREATE INDEX IF NOT EXISTS idx_audit_events_user_id ON audit_events(user_id);
		CREATE INDEX IF NOT EXISTS idx_audit_events_client_id ON audit_events(client_id);
		CREATE INDEX IF NOT EXISTS idx_audit_events_success ON audit_events(success);
		CREATE INDEX IF NOT EXISTS idx_audit_events_composite ON audit_events(event, user_id, timestamp DESC);
	`

	queryCtx, cancel := context.WithTimeout(ctx, constants.DefaultConnectTimeout)
	defer cancel()

	if _, err := ps.pool.Exec(queryCtx, query); err != nil {

		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

func (ps *PostgresStorage) Write(entry *AuditEntry) error {
	if entry == nil {

		return fmt.Errorf("audit entry is nil")
	}

	select {
	case ps.batchCh <- entry:

		return nil
	case <-ps.ctx.Done():

		return fmt.Errorf("storage is closed")
	default:

		return fmt.Errorf("batch channel is full")
	}
}

func (ps *PostgresStorage) WriteBatch(entries []*AuditEntry) error {
	if len(entries) == 0 {

		return nil
	}

	ctx, cancel := context.WithTimeout(ps.ctx, constants.DefaultWriteTimeout)
	defer cancel()

	batch := &pgx.Batch{}

	query := `
		INSERT INTO audit_events (audit_id, timestamp, event, user_id, client_id, ip_address, user_agent, details, success, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (audit_id) DO NOTHING
	`

	for _, entry := range entries {
		var detailsJSON []byte
		if entry.Details != nil {
			var err error
			detailsJSON, err = json.Marshal(entry.Details)
			if err != nil {
				ps.logger.Warning("Failed to marshal details for entry %s: %v", entry.ID, err)
				detailsJSON = []byte("{}")
			}
		}

		batch.Queue(
			query,
			entry.ID,
			entry.Timestamp,
			entry.Event,
			nullStringIfEmpty(entry.UserID),
			nullStringIfEmpty(entry.ClientID),
			nullStringIfEmpty(entry.IP),
			nullStringIfEmpty(entry.UserAgent),
			detailsJSON,
			entry.Success,
			nullStringIfEmpty(entry.Error),
		)
	}

	br := ps.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(entries); i++ {
		_, err := br.Exec()
		if err != nil {
			ps.logger.Warning("Failed to insert audit entry: %v", err)
		}
	}

	return nil
}

func (ps *PostgresStorage) batchWorker() {
	defer ps.wg.Done()

	ticker := time.NewTicker(ps.config.BatchTimeout)
	defer ticker.Stop()

	batch := make([]*AuditEntry, 0, ps.config.BatchSize)

	flush := func() {
		if len(batch) == 0 {

			return
		}

		if err := ps.WriteBatch(batch); err != nil {
			ps.logger.Warning("Failed to write batch: %v", err)
		}

		batch = batch[:0]
	}

	for {
		select {
		case <-ps.stopCh:
			flush()

			return
		case entry := <-ps.batchCh:
			batch = append(batch, entry)

			if len(batch) >= ps.config.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (ps *PostgresStorage) cleanupWorker() {
	defer ps.wg.Done()

	ticker := time.NewTicker(defaultCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ps.stopCh:

			return
		case <-ticker.C:
			if err := ps.cleanupOldEntries(); err != nil {
				ps.logger.Warning("Failed to cleanup old entries: %v", err)
			}
		}
	}
}

func (ps *PostgresStorage) cleanupOldEntries() error {
	ctx, cancel := context.WithTimeout(ps.ctx, constants.DefaultWriteTimeout)
	defer cancel()

	cutoff := time.Now().Add(-ps.config.RetentionPeriod)

	query := `DELETE FROM audit_events WHERE timestamp < $1`

	result, err := ps.pool.Exec(ctx, query, cutoff)
	if err != nil {

		return fmt.Errorf("failed to delete old entries: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected > 0 {
		ps.logger.Info("Cleaned up %d old audit entries", rowsAffected)
	}

	return nil
}

func (ps *PostgresStorage) ReadEntries(limit int, offset int, filter *AuditFilter) ([]AuditEntry, int, error) {
	ctx, cancel := context.WithTimeout(ps.ctx, constants.DefaultReadTimeout)
	defer cancel()

	countQuery := `SELECT COUNT(*) FROM audit_events WHERE 1=1`
	query := `
		SELECT audit_id, timestamp, event, user_id, client_id, ip_address, user_agent, details, success, error
		FROM audit_events
		WHERE 1=1
	`

	args := make([]interface{}, 0)
	argIdx := 1

	if filter != nil {
		if filter.Event != "" {
			query += fmt.Sprintf(" AND event = $%d", argIdx)
			countQuery += fmt.Sprintf(" AND event = $%d", argIdx)
			args = append(args, filter.Event)
			argIdx++
		}
		if filter.UserID != "" {
			query += fmt.Sprintf(" AND user_id = $%d", argIdx)
			countQuery += fmt.Sprintf(" AND user_id = $%d", argIdx)
			args = append(args, filter.UserID)
			argIdx++
		}
		if filter.ClientID != "" {
			query += fmt.Sprintf(" AND client_id = $%d", argIdx)
			countQuery += fmt.Sprintf(" AND client_id = $%d", argIdx)
			args = append(args, filter.ClientID)
			argIdx++
		}
		if filter.Success != nil {
			query += fmt.Sprintf(" AND success = $%d", argIdx)
			countQuery += fmt.Sprintf(" AND success = $%d", argIdx)
			args = append(args, *filter.Success)
			argIdx++
		}
		if !filter.StartTime.IsZero() {
			query += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
			countQuery += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
			args = append(args, filter.StartTime)
			argIdx++
		}
		if !filter.EndTime.IsZero() {
			query += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
			countQuery += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
			args = append(args, filter.EndTime)
			argIdx++
		}
	}

	var total int
	if err := ps.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {

		return nil, 0, fmt.Errorf("failed to count entries: %w", err)
	}

	query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := ps.pool.Query(ctx, query, args...)
	if err != nil {

		return nil, 0, fmt.Errorf("failed to query entries: %w", err)
	}
	defer rows.Close()

	var entries []AuditEntry

	for rows.Next() {
		var entry AuditEntry
		var detailsJSON []byte
		var userID, clientID, ip, userAgent, errorMsg *string

		err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.Event,
			&userID,
			&clientID,
			&ip,
			&userAgent,
			&detailsJSON,
			&entry.Success,
			&errorMsg,
		)
		if err != nil {
			ps.logger.Warning("Failed to scan row: %v", err)

			continue
		}

		if userID != nil {
			entry.UserID = *userID
		}
		if clientID != nil {
			entry.ClientID = *clientID
		}
		if ip != nil {
			entry.IP = *ip
		}
		if userAgent != nil {
			entry.UserAgent = *userAgent
		}
		if errorMsg != nil {
			entry.Error = *errorMsg
		}

		if len(detailsJSON) > 0 {
			if err := json.Unmarshal(detailsJSON, &entry.Details); err != nil {
				ps.logger.Warning("Failed to unmarshal details: %v", err)
			}
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {

		return nil, 0, fmt.Errorf("error iterating rows: %w", err)
	}

	return entries, total, nil
}

func (ps *PostgresStorage) GetStats() (AuditStats, error) {
	ctx, cancel := context.WithTimeout(ps.ctx, constants.DefaultStatsTimeout)
	defer cancel()

	stats := AuditStats{
		EventCounts: make(map[string]int),
	}

	var total int
	if err := ps.pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&total); err != nil {

		return stats, fmt.Errorf("failed to count total entries: %w", err)
	}
	stats.TotalEntries = total

	var successCount int
	if err := ps.pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_events WHERE success = true").Scan(&successCount); err != nil {

		return stats, fmt.Errorf("failed to count successful entries: %w", err)
	}

	if total > 0 {
		stats.SuccessRate = float64(successCount) / float64(total) * PercentageMultiplier
	}

	rows, err := ps.pool.Query(ctx, "SELECT event, COUNT(*) FROM audit_events GROUP BY event")
	if err != nil {

		return stats, fmt.Errorf("failed to query event counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var event string
		var count int

		if err := rows.Scan(&event, &count); err != nil {
			ps.logger.Warning("Failed to scan event count: %v", err)

			continue
		}

		stats.EventCounts[event] = count
	}

	return stats, nil
}

func (ps *PostgresStorage) Flush() error {

	return nil
}

func (ps *PostgresStorage) Close() error {
	ps.cancel()
	close(ps.stopCh)

	done := make(chan struct{})
	go func() {
		ps.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		ps.logger.Debug("PostgreSQL storage workers stopped")
	case <-time.After(constants.DefaultShutdownTimeout):
		ps.logger.Warning("PostgreSQL storage shutdown timeout")
	}

	close(ps.batchCh)

	remaining := make([]*AuditEntry, 0)
	for entry := range ps.batchCh {
		remaining = append(remaining, entry)
	}

	if len(remaining) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultWriteTimeout)
		defer cancel()

		batch := &pgx.Batch{}
		query := `
			INSERT INTO audit_events (audit_id, timestamp, event, user_id, client_id, ip_address, user_agent, details, success, error)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (audit_id) DO NOTHING
		`

		for _, entry := range remaining {
			var detailsJSON []byte
			if entry.Details != nil {
				detailsJSON, _ = json.Marshal(entry.Details)
			}

			batch.Queue(
				query,
				entry.ID,
				entry.Timestamp,
				entry.Event,
				nullStringIfEmpty(entry.UserID),
				nullStringIfEmpty(entry.ClientID),
				nullStringIfEmpty(entry.IP),
				nullStringIfEmpty(entry.UserAgent),
				detailsJSON,
				entry.Success,
				nullStringIfEmpty(entry.Error),
			)
		}

		br := ps.pool.SendBatch(ctx, batch)
		br.Close()
	}

	ps.pool.Close()

	return nil
}

func nullStringIfEmpty(s string) interface{} {
	if s == "" {

		return nil
	}

	return s
}