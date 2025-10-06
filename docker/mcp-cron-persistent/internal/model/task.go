package model

import (
	"time"
)

type Task struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Enabled     bool      `json:"enabled"`
	Command     string    `json:"command,omitempty"`
	Prompt      string    `json:"prompt,omitempty"`
	Schedule    string    `json:"schedule"`
	Timezone    string    `json:"timezone"`

	ChatSessionID         string `json:"chat_session_id,omitempty"`
	CreatedFromMessageID  string `json:"created_from_message_id,omitempty"`
	OutputToChat          bool   `json:"output_to_chat"`
	InheritSessionContext bool   `json:"inherit_session_context"`

	UserID     string   `json:"user_id"`
	Provider   string   `json:"provider,omitempty"`
	Model      string   `json:"model,omitempty"`
	MCPServers []string `json:"mcp_servers,omitempty"`

	Status  string     `json:"status"`
	LastRun *time.Time `json:"last_run,omitempty"`
	NextRun *time.Time `json:"next_run,omitempty"`

	DependsOn         []string               `json:"depends_on,omitempty"`
	TriggerType       string                 `json:"trigger_type,omitempty"`
	WatcherConfig     map[string]interface{} `json:"watcher_config,omitempty"`
	RunOnDemandOnly   bool                   `json:"run_on_demand_only"`

	IsAgent           bool       `json:"is_agent"`
	AgentPersonality  string     `json:"agent_personality,omitempty"`
	MemorySummary     string     `json:"memory_summary,omitempty"`
	LastMemoryUpdate  *time.Time `json:"last_memory_update,omitempty"`

	SkipHolidays       bool                   `json:"skip_holidays"`
	HolidayRegion      string                 `json:"holiday_region,omitempty"`
	MaxExecutionTime   string                 `json:"max_execution_time,omitempty"`
	RetryPolicy        map[string]interface{} `json:"retry_policy,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy string    `json:"created_by"`
}

type TaskRun struct {
	ID         string    `json:"id"`
	TaskID     string    `json:"task_id"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	ExitCode   int       `json:"exit_code"`
	Status     string    `json:"status"`

	PostedToChat  bool   `json:"posted_to_chat"`
	ChatMessageID string `json:"chat_message_id,omitempty"`
	TriggeredBy   string `json:"triggered_by,omitempty"`

	TokensUsed   int     `json:"tokens_used,omitempty"`
	CostEstimate float64 `json:"cost_estimate,omitempty"`
}

type ChatMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
