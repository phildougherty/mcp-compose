package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type PruningStrategy string

const (
	StrategyLRU        PruningStrategy = "lru"
	StrategyImportance PruningStrategy = "importance"
	StrategyHybrid     PruningStrategy = "hybrid"
	StrategyAge        PruningStrategy = "age"
)

type PruningConfig struct {
	Enabled                  bool
	Strategy                 PruningStrategy
	RetentionDays            int
	MinImportanceScore       float64
	MaxMemories              int
	ArchiveBeforeDelete      bool
	DryRun                   bool
	ScheduleCron             string
	PruneEntities            bool
	PruneRelations           bool
	PruneObservations        bool
	LowAccessThreshold       int
	ImportanceDecayFactor    float64
	AgeDecayDays             int
}

type PruningResult struct {
	EntitiesPruned       int       `json:"entities_pruned"`
	RelationsPruned      int       `json:"relations_pruned"`
	ObservationsPruned   int       `json:"observations_pruned"`
	EntitiesArchived     int       `json:"entities_archived"`
	RelationsArchived    int       `json:"relations_archived"`
	ObservationsArchived int       `json:"observations_archived"`
	DurationMs           int64     `json:"duration_ms"`
	Strategy             string    `json:"strategy"`
	Timestamp            time.Time `json:"timestamp"`
	DryRun               bool      `json:"dry_run"`
}

type MemoryPruner struct {
	db     *sql.DB
	config PruningConfig
}

func NewMemoryPruner(db *sql.DB, config PruningConfig) *MemoryPruner {
	if config.RetentionDays == 0 {
		config.RetentionDays = 90
	}
	if config.MinImportanceScore == 0 {
		config.MinImportanceScore = 0.3
	}
	if config.MaxMemories == 0 {
		config.MaxMemories = 100000
	}
	if config.LowAccessThreshold == 0 {
		config.LowAccessThreshold = 5
	}
	if config.ImportanceDecayFactor == 0 {
		config.ImportanceDecayFactor = 0.01
	}
	if config.AgeDecayDays == 0 {
		config.AgeDecayDays = 30
	}
	if config.Strategy == "" {
		config.Strategy = StrategyHybrid
	}

	return &MemoryPruner{
		db:     db,
		config: config,
	}
}

func (p *MemoryPruner) Prune(ctx context.Context) (*PruningResult, error) {
	startTime := time.Now()

	result := &PruningResult{
		Strategy:  string(p.config.Strategy),
		Timestamp: startTime,
		DryRun:    p.config.DryRun,
	}

	switch p.config.Strategy {
	case StrategyLRU:
		if err := p.pruneLRU(ctx, result); err != nil {
			return nil, fmt.Errorf("LRU pruning failed: %w", err)
		}
	case StrategyImportance:
		if err := p.pruneByImportance(ctx, result); err != nil {
			return nil, fmt.Errorf("importance-based pruning failed: %w", err)
		}
	case StrategyHybrid:
		if err := p.pruneHybrid(ctx, result); err != nil {
			return nil, fmt.Errorf("hybrid pruning failed: %w", err)
		}
	case StrategyAge:
		if err := p.pruneByAge(ctx, result); err != nil {
			return nil, fmt.Errorf("age-based pruning failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported pruning strategy: %s", p.config.Strategy)
	}

	result.DurationMs = time.Since(startTime).Milliseconds()

	if !p.config.DryRun {
		if err := p.logPruningRun(ctx, result); err != nil {
			return result, fmt.Errorf("failed to log pruning run: %w", err)
		}
	}

	return result, nil
}

func (p *MemoryPruner) pruneLRU(ctx context.Context, result *PruningResult) error {
	cutoffDate := time.Now().AddDate(0, 0, -p.config.RetentionDays)

	if p.config.PruneEntities {
		count, archived, err := p.pruneLRUTable(ctx, "entities", cutoffDate)
		if err != nil {
			return fmt.Errorf("failed to prune entities: %w", err)
		}
		result.EntitiesPruned = count
		result.EntitiesArchived = archived
	}

	if p.config.PruneRelations {
		count, archived, err := p.pruneLRUTable(ctx, "relations", cutoffDate)
		if err != nil {
			return fmt.Errorf("failed to prune relations: %w", err)
		}
		result.RelationsPruned = count
		result.RelationsArchived = archived
	}

	if p.config.PruneObservations {
		count, archived, err := p.pruneLRUTable(ctx, "observations", cutoffDate)
		if err != nil {
			return fmt.Errorf("failed to prune observations: %w", err)
		}
		result.ObservationsPruned = count
		result.ObservationsArchived = archived
	}

	return nil
}

func (p *MemoryPruner) pruneLRUTable(ctx context.Context, tableName string, cutoffDate time.Time) (int, int, error) {
	archived := 0
	pruned := 0

	if p.config.ArchiveBeforeDelete {
		archiveQuery := fmt.Sprintf(`
			INSERT INTO %s_archive
			SELECT * FROM %s
			WHERE archived = FALSE
				AND last_accessed_at < $1
				AND access_count < $2
		`, tableName, tableName)

		if !p.config.DryRun {
			result, err := p.db.ExecContext(ctx, archiveQuery, cutoffDate, p.config.LowAccessThreshold)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to archive %s: %w", tableName, err)
			}
			rows, _ := result.RowsAffected()
			archived = int(rows)
		} else {
			var count int
			countQuery := fmt.Sprintf(`
				SELECT COUNT(*) FROM %s
				WHERE archived = FALSE
					AND last_accessed_at < $1
					AND access_count < $2
			`, tableName)
			if err := p.db.QueryRowContext(ctx, countQuery, cutoffDate, p.config.LowAccessThreshold).Scan(&count); err != nil {
				return 0, 0, fmt.Errorf("failed to count archivable %s: %w", tableName, err)
			}
			archived = count
		}
	}

	markArchivedQuery := fmt.Sprintf(`
		UPDATE %s
		SET archived = TRUE
		WHERE archived = FALSE
			AND last_accessed_at < $1
			AND access_count < $2
	`, tableName)

	if !p.config.DryRun {
		result, err := p.db.ExecContext(ctx, markArchivedQuery, cutoffDate, p.config.LowAccessThreshold)
		if err != nil {
			return 0, archived, fmt.Errorf("failed to mark %s as archived: %w", tableName, err)
		}
		rows, _ := result.RowsAffected()
		pruned = int(rows)
	} else {
		var count int
		countQuery := fmt.Sprintf(`
			SELECT COUNT(*) FROM %s
			WHERE archived = FALSE
				AND last_accessed_at < $1
				AND access_count < $2
		`, tableName)
		if err := p.db.QueryRowContext(ctx, countQuery, cutoffDate, p.config.LowAccessThreshold).Scan(&count); err != nil {
			return 0, archived, fmt.Errorf("failed to count pruneable %s: %w", tableName, err)
		}
		pruned = count
	}

	return pruned, archived, nil
}

func (p *MemoryPruner) pruneByImportance(ctx context.Context, result *PruningResult) error {
	if p.config.PruneEntities {
		count, archived, err := p.pruneByImportanceTable(ctx, "entities")
		if err != nil {
			return fmt.Errorf("failed to prune entities: %w", err)
		}
		result.EntitiesPruned = count
		result.EntitiesArchived = archived
	}

	if p.config.PruneRelations {
		count, archived, err := p.pruneByImportanceTable(ctx, "relations")
		if err != nil {
			return fmt.Errorf("failed to prune relations: %w", err)
		}
		result.RelationsPruned = count
		result.RelationsArchived = archived
	}

	if p.config.PruneObservations {
		count, archived, err := p.pruneByImportanceTable(ctx, "observations")
		if err != nil {
			return fmt.Errorf("failed to prune observations: %w", err)
		}
		result.ObservationsPruned = count
		result.ObservationsArchived = archived
	}

	return nil
}

func (p *MemoryPruner) pruneByImportanceTable(ctx context.Context, tableName string) (int, int, error) {
	archived := 0
	pruned := 0

	if p.config.ArchiveBeforeDelete {
		archiveQuery := fmt.Sprintf(`
			INSERT INTO %s_archive
			SELECT * FROM %s
			WHERE archived = FALSE
				AND importance_score < $1
		`, tableName, tableName)

		if !p.config.DryRun {
			result, err := p.db.ExecContext(ctx, archiveQuery, p.config.MinImportanceScore)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to archive %s: %w", tableName, err)
			}
			rows, _ := result.RowsAffected()
			archived = int(rows)
		} else {
			var count int
			countQuery := fmt.Sprintf(`
				SELECT COUNT(*) FROM %s
				WHERE archived = FALSE
					AND importance_score < $1
			`, tableName)
			if err := p.db.QueryRowContext(ctx, countQuery, p.config.MinImportanceScore).Scan(&count); err != nil {
				return 0, 0, fmt.Errorf("failed to count archivable %s: %w", tableName, err)
			}
			archived = count
		}
	}

	markArchivedQuery := fmt.Sprintf(`
		UPDATE %s
		SET archived = TRUE
		WHERE archived = FALSE
			AND importance_score < $1
	`, tableName)

	if !p.config.DryRun {
		result, err := p.db.ExecContext(ctx, markArchivedQuery, p.config.MinImportanceScore)
		if err != nil {
			return 0, archived, fmt.Errorf("failed to mark %s as archived: %w", tableName, err)
		}
		rows, _ := result.RowsAffected()
		pruned = int(rows)
	} else {
		var count int
		countQuery := fmt.Sprintf(`
			SELECT COUNT(*) FROM %s
			WHERE archived = FALSE
				AND importance_score < $1
		`, tableName)
		if err := p.db.QueryRowContext(ctx, countQuery, p.config.MinImportanceScore).Scan(&count); err != nil {
			return 0, archived, fmt.Errorf("failed to count pruneable %s: %w", tableName, err)
		}
		pruned = count
	}

	return pruned, archived, nil
}

func (p *MemoryPruner) pruneHybrid(ctx context.Context, result *PruningResult) error {
	if p.config.PruneEntities {
		count, archived, err := p.pruneHybridTable(ctx, "entities")
		if err != nil {
			return fmt.Errorf("failed to prune entities: %w", err)
		}
		result.EntitiesPruned = count
		result.EntitiesArchived = archived
	}

	if p.config.PruneRelations {
		count, archived, err := p.pruneHybridTable(ctx, "relations")
		if err != nil {
			return fmt.Errorf("failed to prune relations: %w", err)
		}
		result.RelationsPruned = count
		result.RelationsArchived = archived
	}

	if p.config.PruneObservations {
		count, archived, err := p.pruneHybridTable(ctx, "observations")
		if err != nil {
			return fmt.Errorf("failed to prune observations: %w", err)
		}
		result.ObservationsPruned = count
		result.ObservationsArchived = archived
	}

	return nil
}

func (p *MemoryPruner) pruneHybridTable(ctx context.Context, tableName string) (int, int, error) {
	archived := 0
	pruned := 0

	cutoffDate := time.Now().AddDate(0, 0, -p.config.RetentionDays)

	condition := fmt.Sprintf(`
		archived = FALSE AND (
			(importance_score < $1 AND last_accessed_at < $2) OR
			(access_count < $3 AND last_accessed_at < $4)
		)
	`)

	if p.config.ArchiveBeforeDelete {
		archiveQuery := fmt.Sprintf(`
			INSERT INTO %s_archive
			SELECT * FROM %s
			WHERE %s
		`, tableName, tableName, condition)

		if !p.config.DryRun {
			result, err := p.db.ExecContext(ctx, archiveQuery,
				p.config.MinImportanceScore, cutoffDate,
				p.config.LowAccessThreshold, cutoffDate)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to archive %s: %w", tableName, err)
			}
			rows, _ := result.RowsAffected()
			archived = int(rows)
		} else {
			var count int
			countQuery := fmt.Sprintf(`
				SELECT COUNT(*) FROM %s
				WHERE %s
			`, tableName, condition)
			if err := p.db.QueryRowContext(ctx, countQuery,
				p.config.MinImportanceScore, cutoffDate,
				p.config.LowAccessThreshold, cutoffDate).Scan(&count); err != nil {
				return 0, 0, fmt.Errorf("failed to count archivable %s: %w", tableName, err)
			}
			archived = count
		}
	}

	markArchivedQuery := fmt.Sprintf(`
		UPDATE %s
		SET archived = TRUE
		WHERE %s
	`, tableName, condition)

	if !p.config.DryRun {
		result, err := p.db.ExecContext(ctx, markArchivedQuery,
			p.config.MinImportanceScore, cutoffDate,
			p.config.LowAccessThreshold, cutoffDate)
		if err != nil {
			return 0, archived, fmt.Errorf("failed to mark %s as archived: %w", tableName, err)
		}
		rows, _ := result.RowsAffected()
		pruned = int(rows)
	} else {
		var count int
		countQuery := fmt.Sprintf(`
			SELECT COUNT(*) FROM %s
			WHERE %s
		`, tableName, condition)
		if err := p.db.QueryRowContext(ctx, countQuery,
			p.config.MinImportanceScore, cutoffDate,
			p.config.LowAccessThreshold, cutoffDate).Scan(&count); err != nil {
			return 0, archived, fmt.Errorf("failed to count pruneable %s: %w", tableName, err)
		}
		pruned = count
	}

	return pruned, archived, nil
}

func (p *MemoryPruner) pruneByAge(ctx context.Context, result *PruningResult) error {
	cutoffDate := time.Now().AddDate(0, 0, -p.config.RetentionDays)

	if p.config.PruneEntities {
		count, archived, err := p.pruneByAgeTable(ctx, "entities", cutoffDate, "created_at")
		if err != nil {
			return fmt.Errorf("failed to prune entities: %w", err)
		}
		result.EntitiesPruned = count
		result.EntitiesArchived = archived
	}

	if p.config.PruneRelations {
		count, archived, err := p.pruneByAgeTable(ctx, "relations", cutoffDate, "created_at")
		if err != nil {
			return fmt.Errorf("failed to prune relations: %w", err)
		}
		result.RelationsPruned = count
		result.RelationsArchived = archived
	}

	if p.config.PruneObservations {
		count, archived, err := p.pruneByAgeTable(ctx, "observations", cutoffDate, "observed_at")
		if err != nil {
			return fmt.Errorf("failed to prune observations: %w", err)
		}
		result.ObservationsPruned = count
		result.ObservationsArchived = archived
	}

	return nil
}

func (p *MemoryPruner) pruneByAgeTable(ctx context.Context, tableName string, cutoffDate time.Time, dateColumn string) (int, int, error) {
	archived := 0
	pruned := 0

	if p.config.ArchiveBeforeDelete {
		archiveQuery := fmt.Sprintf(`
			INSERT INTO %s_archive
			SELECT * FROM %s
			WHERE archived = FALSE
				AND %s < $1
		`, tableName, tableName, dateColumn)

		if !p.config.DryRun {
			result, err := p.db.ExecContext(ctx, archiveQuery, cutoffDate)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to archive %s: %w", tableName, err)
			}
			rows, _ := result.RowsAffected()
			archived = int(rows)
		} else {
			var count int
			countQuery := fmt.Sprintf(`
				SELECT COUNT(*) FROM %s
				WHERE archived = FALSE
					AND %s < $1
			`, tableName, dateColumn)
			if err := p.db.QueryRowContext(ctx, countQuery, cutoffDate).Scan(&count); err != nil {
				return 0, 0, fmt.Errorf("failed to count archivable %s: %w", tableName, err)
			}
			archived = count
		}
	}

	markArchivedQuery := fmt.Sprintf(`
		UPDATE %s
		SET archived = TRUE
		WHERE archived = FALSE
			AND %s < $1
	`, tableName, dateColumn)

	if !p.config.DryRun {
		result, err := p.db.ExecContext(ctx, markArchivedQuery, cutoffDate)
		if err != nil {
			return 0, archived, fmt.Errorf("failed to mark %s as archived: %w", tableName, err)
		}
		rows, _ := result.RowsAffected()
		pruned = int(rows)
	} else {
		var count int
		countQuery := fmt.Sprintf(`
			SELECT COUNT(*) FROM %s
			WHERE archived = FALSE
				AND %s < $1
		`, tableName, dateColumn)
		if err := p.db.QueryRowContext(ctx, countQuery, cutoffDate).Scan(&count); err != nil {
			return 0, archived, fmt.Errorf("failed to count pruneable %s: %w", tableName, err)
		}
		pruned = count
	}

	return pruned, archived, nil
}

func (p *MemoryPruner) logPruningRun(ctx context.Context, result *PruningResult) error {
	metadata, err := json.Marshal(map[string]interface{}{
		"config": p.config,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO pruning_log (
			run_at, strategy, entities_pruned, relations_pruned, observations_pruned,
			entities_archived, relations_archived, observations_archived, duration_ms, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = p.db.ExecContext(ctx, query,
		result.Timestamp, result.Strategy,
		result.EntitiesPruned, result.RelationsPruned, result.ObservationsPruned,
		result.EntitiesArchived, result.RelationsArchived, result.ObservationsArchived,
		result.DurationMs, metadata)

	if err != nil {
		return fmt.Errorf("failed to insert pruning log: %w", err)
	}

	return nil
}

func (p *MemoryPruner) GetPruningHistory(ctx context.Context, limit int) ([]PruningResult, error) {
	if limit == 0 {
		limit = 50
	}

	query := `
		SELECT
			run_at, strategy, entities_pruned, relations_pruned, observations_pruned,
			entities_archived, relations_archived, observations_archived, duration_ms
		FROM pruning_log
		ORDER BY run_at DESC
		LIMIT $1
	`

	rows, err := p.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pruning history: %w", err)
	}

	defer rows.Close()

	var results []PruningResult
	for rows.Next() {
		var result PruningResult
		if err := rows.Scan(
			&result.Timestamp, &result.Strategy,
			&result.EntitiesPruned, &result.RelationsPruned, &result.ObservationsPruned,
			&result.EntitiesArchived, &result.RelationsArchived, &result.ObservationsArchived,
			&result.DurationMs,
		); err != nil {
			return nil, fmt.Errorf("failed to scan pruning history: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pruning history: %w", err)
	}

	return results, nil
}

func (p *MemoryPruner) GetMemoryStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var totalEntities, totalRelations, totalObservations int
	var archivedEntities, archivedRelations, archivedObservations int

	if err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entities WHERE archived = FALSE").Scan(&totalEntities); err != nil {
		return nil, fmt.Errorf("failed to count entities: %w", err)
	}

	if err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM relations WHERE archived = FALSE").Scan(&totalRelations); err != nil {
		return nil, fmt.Errorf("failed to count relations: %w", err)
	}

	if err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM observations WHERE archived = FALSE").Scan(&totalObservations); err != nil {
		return nil, fmt.Errorf("failed to count observations: %w", err)
	}

	if err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entities WHERE archived = TRUE").Scan(&archivedEntities); err != nil {
		return nil, fmt.Errorf("failed to count archived entities: %w", err)
	}

	if err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM relations WHERE archived = TRUE").Scan(&archivedRelations); err != nil {
		return nil, fmt.Errorf("failed to count archived relations: %w", err)
	}

	if err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM observations WHERE archived = TRUE").Scan(&archivedObservations); err != nil {
		return nil, fmt.Errorf("failed to count archived observations: %w", err)
	}

	stats["total_entities"] = totalEntities
	stats["total_relations"] = totalRelations
	stats["total_observations"] = totalObservations
	stats["archived_entities"] = archivedEntities
	stats["archived_relations"] = archivedRelations
	stats["archived_observations"] = archivedObservations
	stats["total_active"] = totalEntities + totalRelations + totalObservations
	stats["total_archived"] = archivedEntities + archivedRelations + archivedObservations

	return stats, nil
}

func (p *MemoryPruner) UpdateImportanceScores(ctx context.Context) error {
	updateQuery := `
		UPDATE entities
		SET importance_score = LEAST(1.0,
			importance_score *
			(1.0 + LOG(1 + access_count) * $1) *
			(1.0 / (1.0 + EXTRACT(EPOCH FROM (NOW() - last_accessed_at)) / 86400.0 * $2))
		)
		WHERE archived = FALSE
	`

	_, err := p.db.ExecContext(ctx, updateQuery, p.config.ImportanceDecayFactor, p.config.ImportanceDecayFactor)
	if err != nil {
		return fmt.Errorf("failed to update entity importance scores: %w", err)
	}

	return nil
}

func (p *MemoryPruner) CleanupArchive(ctx context.Context, daysOld int) error {
	cutoffDate := time.Now().AddDate(0, 0, -daysOld)

	tables := []string{"entities_archive", "relations_archive", "observations_archive"}

	for _, table := range tables {
		deleteQuery := fmt.Sprintf(`DELETE FROM %s WHERE created_at < $1`, table)

		result, err := p.db.ExecContext(ctx, deleteQuery, cutoffDate)
		if err != nil {
			return fmt.Errorf("failed to cleanup %s: %w", table, err)
		}

		rows, _ := result.RowsAffected()
		if rows > 0 {
		}
	}

	return nil
}