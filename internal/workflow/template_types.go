package workflow

import (
	"encoding/json"
	"time"
)

type Template struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Category           string          `json:"category"`
	Author             string          `json:"author"`
	Thumbnail          string          `json:"thumbnail,omitempty"`
	Tags               []string        `json:"tags"`
	Version            string          `json:"version"`
	Downloads          int             `json:"downloads"`
	Rating             float64         `json:"rating"`
	WorkflowDefinition json.RawMessage `json:"workflow_definition"`
	RequiredServers    []string        `json:"required_servers"`
	EstimatedCost      string          `json:"estimated_cost,omitempty"`
	Difficulty         string          `json:"difficulty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type TemplateFilter struct {
	Category   string
	Difficulty string
	Search     string
	Tags       []string
	SortBy     string
	Limit      int
	Offset     int
}

type InstallTemplateRequest struct {
	TemplateID string                 `json:"template_id"`
	Parameters map[string]interface{} `json:"parameters"`
}

type InstallTemplateResponse struct {
	WorkflowID string `json:"workflow_id"`
	Message    string `json:"message"`
}

type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

const (
	CategoryDataEngineering     = "Data Engineering"
	CategoryMonitoringAlerts    = "Monitoring & Alerts"
	CategoryContentGeneration   = "Content Generation"
	CategoryCustomerSupport     = "Customer Support"
	CategoryMarketingAutomation = "Marketing Automation"
	CategoryDevOps              = "DevOps"

	DifficultyBeginner     = "beginner"
	DifficultyIntermediate = "intermediate"
	DifficultyAdvanced     = "advanced"

	SortByPopularity = "popularity"
	SortByRating     = "rating"
	SortByRecent     = "recent"
)
