package studio

import (
	"time"

	"github.com/soasurs/adk/model"
)

type AgentSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type AppSummary struct {
	Name              string                 `json:"name"`
	AgentCount        int                    `json:"agent_count"`
	HasSessionService bool                   `json:"has_session_service"`
	SessionBackend    *SessionBackendSummary `json:"session_backend,omitempty"`
}

type SessionBackendSummary struct {
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
	Type      string          `json:"type"`
	RunID     string          `json:"run_id"`
	SessionID string          `json:"session_id,omitempty"`
	Event     *model.Event    `json:"event,omitempty"`
	Trace     *RunTraceRecord `json:"trace,omitempty"`
	Failure   *RunFailure     `json:"failure,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// RunTraceRecord is the stable wire representation of one ADK runtime span
// callback collected during a Studio run.
type RunTraceRecord struct {
	Phase        string        `json:"phase"`
	Kind         string        `json:"kind"`
	Time         time.Time     `json:"time"`
	Duration     time.Duration `json:"duration,omitempty"`
	RuntimeRunID string        `json:"runtime_run_id,omitempty"`
	TurnID       string        `json:"turn_id,omitempty"`
	SessionID    string        `json:"session_id,omitempty"`
	AppID        string        `json:"app_id,omitempty"`
	UserID       string        `json:"user_id,omitempty"`

	AgentName string `json:"agent_name,omitempty"`
	Model     string `json:"model,omitempty"`
	Iteration int    `json:"iteration,omitempty"`
	Stream    bool   `json:"stream,omitempty"`

	EventID     string `json:"event_id,omitempty"`
	EventAuthor string `json:"event_author,omitempty"`
	EventRole   string `json:"event_role,omitempty"`
	EventCount  int    `json:"event_count,omitempty"`
	Partial     bool   `json:"partial,omitempty"`

	ToolName   string `json:"tool_name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	ToolIndex  *int   `json:"tool_index,omitempty"`

	FinishReason     string         `json:"finish_reason,omitempty"`
	PromptTokens     int64          `json:"prompt_tokens,omitempty"`
	CompletionTokens int64          `json:"completion_tokens,omitempty"`
	TotalTokens      int64          `json:"total_tokens,omitempty"`
	PartialResponses int            `json:"partial_responses,omitempty"`
	StoppedEarly     bool           `json:"stopped_early,omitempty"`
	IsError          bool           `json:"is_error,omitempty"`
	Error            string         `json:"error,omitempty"`
	Attributes       map[string]any `json:"attributes,omitempty"`
}

// RunFailure describes a terminal run failure without requiring clients to
// parse the backward-compatible Error string.
type RunFailure struct {
	Code            string               `json:"code"`
	Message         string               `json:"message"`
	SessionID       string               `json:"session_id,omitempty"`
	SourceTurnID    string               `json:"source_turn_id,omitempty"`
	SourceEventID   string               `json:"source_event_id,omitempty"`
	UnresolvedTools []RunFailureToolCall `json:"unresolved_tools,omitempty"`
}

// RunFailureToolCall identifies an unresolved tool call without exposing its
// arguments.
type RunFailureToolCall struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}
