package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type Storage struct {
	db *sql.DB
}

func NewStorage(db *sql.DB) (*Storage, error) {
	storage := &Storage{db: db}
	if err := storage.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return storage, nil
}

func (s *Storage) initTables() error {
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS workflows`,
		`CREATE TABLE IF NOT EXISTS workflows.workflows (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			version INTEGER DEFAULT 1,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			created_by TEXT,
			metadata JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS workflows.workflow_nodes (
			id TEXT PRIMARY KEY,
			workflow_id TEXT REFERENCES workflows.workflows(id) ON DELETE CASCADE,
			type TEXT NOT NULL,
			position_x DOUBLE PRECISION,
			position_y DOUBLE PRECISION,
			data JSONB NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS workflows.workflow_edges (
			id TEXT PRIMARY KEY,
			workflow_id TEXT REFERENCES workflows.workflows(id) ON DELETE CASCADE,
			source TEXT NOT NULL,
			target TEXT NOT NULL,
			source_handle TEXT,
			target_handle TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS workflows.workflow_executions (
			id TEXT PRIMARY KEY,
			workflow_id TEXT REFERENCES workflows.workflows(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			started_at TIMESTAMPTZ DEFAULT NOW(),
			completed_at TIMESTAMPTZ,
			error TEXT,
			result JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS workflows.node_execution_states (
			id SERIAL PRIMARY KEY,
			execution_id TEXT REFERENCES workflows.workflow_executions(id) ON DELETE CASCADE,
			node_id TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TIMESTAMPTZ DEFAULT NOW(),
			completed_at TIMESTAMPTZ,
			error TEXT,
			output JSONB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workflows_created_at ON workflows.workflows(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_nodes_workflow_id ON workflows.workflow_nodes(workflow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_edges_workflow_id ON workflows.workflow_edges(workflow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow_id ON workflows.workflow_executions(workflow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_executions_status ON workflows.workflow_executions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_node_execution_states_execution_id ON workflows.node_execution_states(execution_id)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

func (s *Storage) CreateWorkflow(ctx context.Context, workflow *Workflow) error {
	validator := NewValidator()
	validationResult := validator.Validate(workflow)

	if !validationResult.Valid {
		return &WorkflowValidationError{Result: validationResult}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if workflow.ID == "" {
		workflow.ID = uuid.New().String()
	}
	if workflow.CreatedAt.IsZero() {
		workflow.CreatedAt = time.Now()
	}
	workflow.UpdatedAt = time.Now()

	metadataJSON, err := json.Marshal(workflow.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `INSERT INTO workflows.workflows (id, name, description, version, created_at, updated_at, created_by, metadata)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = tx.ExecContext(ctx, query,
		workflow.ID,
		workflow.Name,
		workflow.Description,
		workflow.Version,
		workflow.CreatedAt,
		workflow.UpdatedAt,
		workflow.CreatedBy,
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	for _, node := range workflow.Nodes {
		nodeQuery := `INSERT INTO workflows.workflow_nodes (id, workflow_id, type, position_x, position_y, data)
		              VALUES ($1, $2, $3, $4, $5, $6)`

		_, err = tx.ExecContext(ctx, nodeQuery,
			node.ID,
			workflow.ID,
			node.Type,
			node.Position.X,
			node.Position.Y,
			node.Data,
		)
		if err != nil {
			return fmt.Errorf("failed to create node: %w", err)
		}
	}

	for _, edge := range workflow.Edges {
		edgeQuery := `INSERT INTO workflows.workflow_edges (id, workflow_id, source, target, source_handle, target_handle)
		              VALUES ($1, $2, $3, $4, $5, $6)`

		_, err = tx.ExecContext(ctx, edgeQuery,
			edge.ID,
			workflow.ID,
			edge.Source,
			edge.Target,
			edge.SourceHandle,
			edge.TargetHandle,
		)
		if err != nil {
			return fmt.Errorf("failed to create edge: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Storage) GetWorkflow(ctx context.Context, workflowID string) (*Workflow, error) {
	workflow := &Workflow{}
	var metadataJSON []byte
	var createdBy sql.NullString

	query := `SELECT id, name, description, version, created_at, updated_at, created_by, metadata
	          FROM workflows.workflows WHERE id = $1`

	err := s.db.QueryRowContext(ctx, query, workflowID).Scan(
		&workflow.ID,
		&workflow.Name,
		&workflow.Description,
		&workflow.Version,
		&workflow.CreatedAt,
		&workflow.UpdatedAt,
		&createdBy,
		&metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workflow not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	if createdBy.Valid {
		workflow.CreatedBy = &createdBy.String
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &workflow.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	nodesQuery := `SELECT id, type, position_x, position_y, data
	               FROM workflows.workflow_nodes WHERE workflow_id = $1`

	rows, err := s.db.QueryContext(ctx, nodesQuery, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		node := WorkflowNode{}
		err := rows.Scan(
			&node.ID,
			&node.Type,
			&node.Position.X,
			&node.Position.Y,
			&node.Data,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		workflow.Nodes = append(workflow.Nodes, node)
	}

	edgesQuery := `SELECT id, source, target, source_handle, target_handle
	               FROM workflows.workflow_edges WHERE workflow_id = $1`

	edgeRows, err := s.db.QueryContext(ctx, edgesQuery, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		edge := WorkflowEdge{}
		err := edgeRows.Scan(
			&edge.ID,
			&edge.Source,
			&edge.Target,
			&edge.SourceHandle,
			&edge.TargetHandle,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan edge: %w", err)
		}
		workflow.Edges = append(workflow.Edges, edge)
	}

	return workflow, nil
}

func (s *Storage) ListWorkflows(ctx context.Context, limit int) ([]*Workflow, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, name, description, version, created_at, updated_at, created_by, metadata
	          FROM workflows.workflows
	          ORDER BY created_at DESC
	          LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	defer rows.Close()

	var workflows []*Workflow
	for rows.Next() {
		workflow := &Workflow{}
		var metadataJSON []byte
		var createdBy sql.NullString

		err := rows.Scan(
			&workflow.ID,
			&workflow.Name,
			&workflow.Description,
			&workflow.Version,
			&workflow.CreatedAt,
			&workflow.UpdatedAt,
			&createdBy,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workflow: %w", err)
		}

		if createdBy.Valid {
			workflow.CreatedBy = &createdBy.String
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &workflow.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		workflows = append(workflows, workflow)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate workflows: %w", err)
	}

	return workflows, nil
}

func (s *Storage) UpdateWorkflow(ctx context.Context, workflow *Workflow) error {
	validator := NewValidator()
	validationResult := validator.Validate(workflow)

	if !validationResult.Valid {
		return &WorkflowValidationError{Result: validationResult}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	workflow.UpdatedAt = time.Now()
	workflow.Version++

	metadataJSON, err := json.Marshal(workflow.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `UPDATE workflows.workflows
	          SET name = $1, description = $2, version = $3, updated_at = $4, metadata = $5
	          WHERE id = $6`

	result, err := tx.ExecContext(ctx, query,
		workflow.Name,
		workflow.Description,
		workflow.Version,
		workflow.UpdatedAt,
		metadataJSON,
		workflow.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update workflow: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("workflow not found")
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM workflows.workflow_nodes WHERE workflow_id = $1`, workflow.ID); err != nil {
		return fmt.Errorf("failed to delete old nodes: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM workflows.workflow_edges WHERE workflow_id = $1`, workflow.ID); err != nil {
		return fmt.Errorf("failed to delete old edges: %w", err)
	}

	for _, node := range workflow.Nodes {
		nodeQuery := `INSERT INTO workflows.workflow_nodes (id, workflow_id, type, position_x, position_y, data)
		              VALUES ($1, $2, $3, $4, $5, $6)`

		_, err = tx.ExecContext(ctx, nodeQuery,
			node.ID,
			workflow.ID,
			node.Type,
			node.Position.X,
			node.Position.Y,
			node.Data,
		)
		if err != nil {
			return fmt.Errorf("failed to create node: %w", err)
		}
	}

	for _, edge := range workflow.Edges {
		edgeQuery := `INSERT INTO workflows.workflow_edges (id, workflow_id, source, target, source_handle, target_handle)
		              VALUES ($1, $2, $3, $4, $5, $6)`

		_, err = tx.ExecContext(ctx, edgeQuery,
			edge.ID,
			workflow.ID,
			edge.Source,
			edge.Target,
			edge.SourceHandle,
			edge.TargetHandle,
		)
		if err != nil {
			return fmt.Errorf("failed to create edge: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Storage) DeleteWorkflow(ctx context.Context, workflowID string) error {
	query := `DELETE FROM workflows.workflows WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, workflowID)
	if err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("workflow not found")
	}

	return nil
}

func (s *Storage) CreateExecution(ctx context.Context, execution *WorkflowExecution) error {
	if execution.ID == "" {
		execution.ID = uuid.New().String()
	}
	if execution.StartedAt.IsZero() {
		execution.StartedAt = time.Now()
	}

	resultJSON, err := json.Marshal(execution.Result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	query := `INSERT INTO workflows.workflow_executions (id, workflow_id, status, started_at, completed_at, error, result)
	          VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = s.db.ExecContext(ctx, query,
		execution.ID,
		execution.WorkflowID,
		execution.Status,
		execution.StartedAt,
		execution.CompletedAt,
		execution.Error,
		resultJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to create execution: %w", err)
	}

	return nil
}

func (s *Storage) UpdateExecution(ctx context.Context, execution *WorkflowExecution) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	resultJSON, err := json.Marshal(execution.Result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	query := `UPDATE workflows.workflow_executions
	          SET status = $1, completed_at = $2, error = $3, result = $4
	          WHERE id = $5`

	result, err := tx.ExecContext(ctx, query,
		execution.Status,
		execution.CompletedAt,
		execution.Error,
		resultJSON,
		execution.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("execution not found")
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM workflows.node_execution_states WHERE execution_id = $1`, execution.ID); err != nil {
		return fmt.Errorf("failed to delete old node states: %w", err)
	}

	for _, nodeState := range execution.NodeStates {
		outputJSON, err := json.Marshal(nodeState.Output)
		if err != nil {
			return fmt.Errorf("failed to marshal node output: %w", err)
		}

		nodeQuery := `INSERT INTO workflows.node_execution_states
		              (execution_id, node_id, status, started_at, completed_at, error, output)
		              VALUES ($1, $2, $3, $4, $5, $6, $7)`

		_, err = tx.ExecContext(ctx, nodeQuery,
			execution.ID,
			nodeState.NodeID,
			nodeState.Status,
			nodeState.StartedAt,
			nodeState.CompletedAt,
			nodeState.Error,
			outputJSON,
		)
		if err != nil {
			return fmt.Errorf("failed to insert node state: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Storage) ListExecutions(ctx context.Context, workflowID string, limit int) ([]WorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, status, started_at, completed_at, error, result
		FROM workflows.workflow_executions
		WHERE workflow_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, query, workflowID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query executions: %w", err)
	}
	defer rows.Close()

	var executions []WorkflowExecution
	for rows.Next() {
		var exec WorkflowExecution
		var completedAt sql.NullTime
		var errorStr sql.NullString
		var resultJSON []byte

		err := rows.Scan(
			&exec.ID,
			&exec.WorkflowID,
			&exec.Status,
			&exec.StartedAt,
			&completedAt,
			&errorStr,
			&resultJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}

		if completedAt.Valid {
			exec.CompletedAt = &completedAt.Time
		}

		if errorStr.Valid {
			exec.Error = errorStr.String
		}

		if len(resultJSON) > 0 {
			if err := json.Unmarshal(resultJSON, &exec.Result); err != nil {
				return nil, fmt.Errorf("failed to unmarshal result: %w", err)
			}
		}

		nodeStatesQuery := `SELECT node_id, status, started_at, completed_at, error, output
		                    FROM workflows.node_execution_states
		                    WHERE execution_id = $1
		                    ORDER BY started_at ASC`

		nodeRows, err := s.db.QueryContext(ctx, nodeStatesQuery, exec.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query node states: %w", err)
		}

		exec.NodeStates = []NodeExecutionState{}
		for nodeRows.Next() {
			var nodeState NodeExecutionState
			var nodeCompletedAt sql.NullTime
			var nodeErrorStr sql.NullString
			var outputJSON []byte

			err := nodeRows.Scan(
				&nodeState.NodeID,
				&nodeState.Status,
				&nodeState.StartedAt,
				&nodeCompletedAt,
				&nodeErrorStr,
				&outputJSON,
			)
			if err != nil {
				nodeRows.Close()
				return nil, fmt.Errorf("failed to scan node state: %w", err)
			}

			if nodeCompletedAt.Valid {
				nodeState.CompletedAt = &nodeCompletedAt.Time
			}

			if nodeErrorStr.Valid {
				nodeState.Error = nodeErrorStr.String
			}

			if len(outputJSON) > 0 {
				if err := json.Unmarshal(outputJSON, &nodeState.Output); err != nil {
					nodeRows.Close()
					return nil, fmt.Errorf("failed to unmarshal node output: %w", err)
				}
			}

			exec.NodeStates = append(exec.NodeStates, nodeState)
		}
		nodeRows.Close()

		if err := nodeRows.Err(); err != nil {
			return nil, fmt.Errorf("failed to iterate node states: %w", err)
		}

		executions = append(executions, exec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return executions, nil
}

func (s *Storage) GetExecution(ctx context.Context, executionID string) (*WorkflowExecution, error) {
	execution := &WorkflowExecution{}
	var resultJSON []byte

	query := `SELECT id, workflow_id, status, started_at, completed_at, error, result
	          FROM workflows.workflow_executions WHERE id = $1`

	err := s.db.QueryRowContext(ctx, query, executionID).Scan(
		&execution.ID,
		&execution.WorkflowID,
		&execution.Status,
		&execution.StartedAt,
		&execution.CompletedAt,
		&execution.Error,
		&resultJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("execution not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	if len(resultJSON) > 0 {
		if err := json.Unmarshal(resultJSON, &execution.Result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	nodeStatesQuery := `SELECT node_id, status, started_at, completed_at, error, output
	                    FROM workflows.node_execution_states
	                    WHERE execution_id = $1
	                    ORDER BY started_at ASC`

	rows, err := s.db.QueryContext(ctx, nodeStatesQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query node states: %w", err)
	}
	defer rows.Close()

	execution.NodeStates = []NodeExecutionState{}
	for rows.Next() {
		var nodeState NodeExecutionState
		var completedAt sql.NullTime
		var errorStr sql.NullString
		var outputJSON []byte

		err := rows.Scan(
			&nodeState.NodeID,
			&nodeState.Status,
			&nodeState.StartedAt,
			&completedAt,
			&errorStr,
			&outputJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node state: %w", err)
		}

		if completedAt.Valid {
			nodeState.CompletedAt = &completedAt.Time
		}

		if errorStr.Valid {
			nodeState.Error = errorStr.String
		}

		if len(outputJSON) > 0 {
			if err := json.Unmarshal(outputJSON, &nodeState.Output); err != nil {
				return nil, fmt.Errorf("failed to unmarshal node output: %w", err)
			}
		}

		execution.NodeStates = append(execution.NodeStates, nodeState)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate node states: %w", err)
	}

	return execution, nil
}

func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}

	return nil
}
