package model

import "time"

type Task struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	Type                   string    `json:"type"`
	Enabled                bool      `json:"enabled"`
	Command                string    `json:"command,omitempty"`
	Prompt                 string    `json:"prompt,omitempty"`
	Schedule               string    `json:"schedule"`
	Timezone               string    `json:"timezone"`
	ChatSessionID          string    `json:"chat_session_id,omitempty"`
	OutputToChat           bool      `json:"output_to_chat"`
	InheritSessionContext  bool      `json:"inherit_session_context"`
	Provider               string    `json:"provider"`
	Model                  string    `json:"model"`
	MCPServers             []string  `json:"mcp_servers"`
	Status                 string    `json:"status"`
	LastRun                time.Time `json:"last_run,omitempty"`
	NextRun                time.Time `json:"next_run,omitempty"`
	TriggerType            string    `json:"trigger_type"`
	IsAgent                bool      `json:"is_agent"`
	AgentPersonality       string    `json:"agent_personality,omitempty"`
	UserID                 string    `json:"user_id"`
	CreatedBy              string    `json:"created_by"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type TaskRun struct {
	ID            string    `json:"id"`
	TaskID        string    `json:"task_id"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Output        string    `json:"output"`
	Error         string    `json:"error,omitempty"`
	ExitCode      int       `json:"exit_code"`
	Status        string    `json:"status"`
	PostedToChat  bool      `json:"posted_to_chat"`
	ChatMessageID string    `json:"chat_message_id,omitempty"`
	TriggeredBy   string    `json:"triggered_by"`
	TokensUsed    int       `json:"tokens_used,omitempty"`
	CostEstimate  float64   `json:"cost_estimate,omitempty"`
}
