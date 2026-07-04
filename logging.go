package studio

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/soasurs/adk/model"
)

// LogLevel controls which Studio logs are emitted by the default logger.
type LogLevel string

const (
	// LogLevelDebug enables debug, info, warn, and error logs.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo enables info, warn, and error logs.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn enables warn and error logs.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError enables only error logs.
	LogLevelError LogLevel = "error"
	// LogLevelOff disables Studio logs emitted through the default logger.
	LogLevelOff LogLevel = "off"
)

const logLevelOff slog.Level = 100

// NewLogger creates Studio's default text logger for the supplied level.
// A nil writer logs to stderr.
func NewLogger(w io.Writer, level LogLevel) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level.slogLevel(),
	}))
}

func (l LogLevel) slogLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(string(l))) {
	case "", string(LogLevelInfo):
		return slog.LevelInfo
	case string(LogLevelDebug):
		return slog.LevelDebug
	case string(LogLevelWarn), "warning":
		return slog.LevelWarn
	case string(LogLevelError):
		return slog.LevelError
	case string(LogLevelOff), "none", "disabled":
		return logLevelOff
	default:
		return slog.LevelInfo
	}
}

func logRunEvent(ctx context.Context, logger *slog.Logger, runID, sessionID, agentID string, event *model.Event) {
	if logger == nil || !logger.Enabled(ctx, slog.LevelInfo) {
		return
	}

	attrs := []any{
		"run_id", runID,
		"session_id", sessionID,
		"agent_id", agentID,
	}
	if event != nil {
		attrs = append(attrs,
			"event_id", event.ID,
			"author", event.Author,
			"role", string(event.Content.Role),
			"partial", event.Partial,
			"finish_reason", string(event.FinishReason),
			"tool_calls", len(event.Content.ToolCalls),
		)
		if event.Usage != nil {
			attrs = append(attrs,
				"prompt_tokens", event.Usage.PromptTokens,
				"completion_tokens", event.Usage.CompletionTokens,
				"total_tokens", event.Usage.TotalTokens,
			)
		}
		if data, err := json.Marshal(event); err == nil {
			attrs = append(attrs, "event", string(data))
		} else {
			attrs = append(attrs, "event_marshal_error", err.Error())
		}
	}

	logger.InfoContext(ctx, "adk studio event", attrs...)
}
