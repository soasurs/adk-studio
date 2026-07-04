package studio

import "github.com/soasurs/adk/model"

type AgentSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type SessionSummary struct {
	SessionID string `json:"session_id"`
	AppID     string `json:"app_id"`
	UserID    string `json:"user_id"`
}

type RunRequest struct {
	AgentID   string        `json:"agent_id"`
	AppName   string        `json:"app_name,omitempty"`
	UserID    string        `json:"user_id"`
	SessionID string        `json:"session_id,omitempty"`
	Input     model.Content `json:"input"`
	Overrides RunOverrides  `json:"overrides"`
}

type RunOverrides struct {
	Model         *string  `json:"model,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	MaxIterations *int     `json:"max_iterations,omitempty"`
	Instruction   *string  `json:"instruction,omitempty"`
	DisabledTools []string `json:"disabled_tools,omitempty"`
	Stream        *bool    `json:"stream,omitempty"`
}

type RunResponse struct {
	RunID     string           `json:"run_id"`
	SessionID string           `json:"session_id"`
	Events    []RunStreamEvent `json:"events"`
	Error     string           `json:"error,omitempty"`
}

type RunStreamEvent struct {
	Type      string       `json:"type"`
	RunID     string       `json:"run_id"`
	SessionID string       `json:"session_id,omitempty"`
	Event     *model.Event `json:"event,omitempty"`
	Error     string       `json:"error,omitempty"`
}
