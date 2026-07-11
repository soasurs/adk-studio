package studio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	adkagent "github.com/soasurs/adk/agent"
	"github.com/soasurs/adk/runner"
	"github.com/soasurs/adk/session"
	adktrace "github.com/soasurs/adk/trace"
)

const runFeedCapacity = 64

type runExecution struct {
	events <-chan RunStreamEvent
	done   <-chan error
	cancel context.CancelFunc
}

func (e *runExecution) cancelAndWait() {
	e.cancel()
	<-e.done
}

func (h *Handler) startRun(
	parent context.Context,
	agent adkagent.Agent,
	sessionService session.SessionService,
	runID string,
	req RunRequest,
) (*runExecution, error) {
	ctx, cancel := context.WithCancel(parent)
	feed := make(chan RunStreamEvent, runFeedCapacity)
	done := make(chan error, 1)
	emit := func(event RunStreamEvent) bool {
		event.RunID = runID
		if event.SessionID == "" {
			event.SessionID = req.SessionID
		}
		select {
		case feed <- event:
			return true
		case <-ctx.Done():
			return false
		}
	}
	tracer := newRunTracer(runID, req.SessionID, emit, h.app.tracerForRun())
	adkRunner, err := runner.New(agent, sessionService, runner.WithTracer(tracer))
	if err != nil {
		cancel()
		return nil, err
	}

	go func() {
		var completionErr error
		defer func() {
			close(feed)
			done <- completionErr
			close(done)
		}()

		var runErr error
		eventCount := 0
		for event, err := range adkRunner.Run(ctx, req.SessionID, req.Input) {
			if err != nil {
				runErr = err
				break
			}
			logRunEvent(ctx, h.app.loggerForRun(), runID, req.SessionID, req.AgentID, event)
			eventCount++
			eventType := "event"
			if event.Partial {
				eventType = "partial"
			}
			if !emit(RunStreamEvent{Type: eventType, Event: event}) {
				completionErr = context.Cause(ctx)
				if completionErr == nil {
					completionErr = context.Canceled
				}
				return
			}
		}

		if runErr != nil {
			completionErr = runErr
			if ctx.Err() != nil {
				return
			}
			h.app.loggerForRun().ErrorContext(ctx, "adk studio run failed",
				"run_id", runID,
				"agent_id", req.AgentID,
				"session_id", req.SessionID,
				"event_count", eventCount,
				"error", runErr,
			)
			emit(RunStreamEvent{
				Type:    "error",
				Error:   runErr.Error(),
				Failure: runFailure(runErr, req.SessionID),
			})
			return
		}
		if completionErr = context.Cause(ctx); completionErr != nil {
			return
		}

		h.app.loggerForRun().InfoContext(ctx, "adk studio run completed",
			"run_id", runID,
			"agent_id", req.AgentID,
			"session_id", req.SessionID,
			"event_count", eventCount,
		)
		if !emit(RunStreamEvent{Type: "done"}) {
			completionErr = context.Cause(ctx)
			if completionErr == nil {
				completionErr = context.Canceled
			}
		}
	}()

	return &runExecution{events: feed, done: done, cancel: cancel}, nil
}

func runFailure(err error, sessionID string) *RunFailure {
	failure := &RunFailure{
		Code:      "run_failed",
		Message:   err.Error(),
		SessionID: sessionID,
	}
	var unknown *runner.ToolExecutionUnknownError
	if !errors.As(err, &unknown) {
		return failure
	}

	failure.Code = "tool_execution_unknown"
	failure.SessionID = unknown.SessionID
	failure.SourceTurnID = unknown.TurnID
	if unknown.EventID != 0 {
		failure.SourceEventID = strconv.FormatInt(unknown.EventID, 10)
	}
	failure.UnresolvedTools = make([]RunFailureToolCall, 0, len(unknown.ToolCalls))
	for _, call := range unknown.ToolCalls {
		failure.UnresolvedTools = append(failure.UnresolvedTools, RunFailureToolCall{
			ID:   call.ID,
			Name: call.Name,
		})
	}
	return failure
}

type runTracer struct {
	runID     string
	sessionID string
	emit      func(RunStreamEvent) bool
	host      adktrace.Tracer
}

func newRunTracer(runID, sessionID string, emit func(RunStreamEvent) bool, host adktrace.Tracer) adktrace.Tracer {
	return &runTracer{runID: runID, sessionID: sessionID, emit: emit, host: host}
}

func (t *runTracer) Start(ctx context.Context, event adktrace.Event) (context.Context, adktrace.Span) {
	hostCtx := ctx
	var hostSpan adktrace.Span
	if t.host != nil {
		hostCtx, hostSpan = t.host.Start(ctx, event)
		if hostCtx == nil {
			hostCtx = ctx
		}
	}
	started := normalizeTraceEvent(event, time.Time{}, "start")
	t.emit(RunStreamEvent{
		Type:      "trace",
		RunID:     t.runID,
		SessionID: t.sessionID,
		Trace:     traceRecord("start", started),
	})
	return hostCtx, &runSpan{tracer: t, host: hostSpan, started: started}
}

type runSpan struct {
	tracer  *runTracer
	host    adktrace.Span
	started adktrace.Event
	once    sync.Once
}

func (s *runSpan) AddEvent(ctx context.Context, event adktrace.Event) {
	if s.host != nil {
		s.host.AddEvent(ctx, event)
	}
	event = mergeTraceEvent(s.started, event)
	event = normalizeTraceEvent(event, s.started.Time, "event")
	s.tracer.emit(RunStreamEvent{
		Type:      "trace",
		RunID:     s.tracer.runID,
		SessionID: s.tracer.sessionID,
		Trace:     traceRecord("event", event),
	})
}

func (s *runSpan) End(ctx context.Context, event adktrace.Event) {
	s.once.Do(func() {
		if s.host != nil {
			s.host.End(ctx, event)
		}
		event = mergeTraceEvent(s.started, event)
		event = normalizeTraceEvent(event, s.started.Time, "end")
		s.tracer.emit(RunStreamEvent{
			Type:      "trace",
			RunID:     s.tracer.runID,
			SessionID: s.tracer.sessionID,
			Trace:     traceRecord("end", event),
		})
	})
}

func normalizeTraceEvent(event adktrace.Event, started time.Time, phase string) adktrace.Event {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	if phase == "end" && event.Duration == 0 && !started.IsZero() {
		event.Duration = event.Time.Sub(started)
	}
	return event
}

func mergeTraceEvent(started, event adktrace.Event) adktrace.Event {
	if event.Kind == "" {
		event.Kind = started.Kind
	}
	if event.RunID == "" {
		event.RunID = started.RunID
	}
	if event.TurnID == "" {
		event.TurnID = started.TurnID
	}
	if event.SessionID == "" {
		event.SessionID = started.SessionID
	}
	if event.AppID == "" {
		event.AppID = started.AppID
	}
	if event.UserID == "" {
		event.UserID = started.UserID
	}
	if event.AgentName == "" {
		event.AgentName = started.AgentName
	}
	if event.Model == "" {
		event.Model = started.Model
	}
	if event.ToolName == "" {
		event.ToolName = started.ToolName
	}
	if event.ToolCallID == "" {
		event.ToolCallID = started.ToolCallID
	}
	if event.Iteration == 0 {
		event.Iteration = started.Iteration
	}
	if event.ToolIndex == 0 {
		event.ToolIndex = started.ToolIndex
	}
	if !event.Stream {
		event.Stream = started.Stream
	}
	return event
}

func traceRecord(phase string, event adktrace.Event) *RunTraceRecord {
	record := &RunTraceRecord{
		Phase:            phase,
		Kind:             string(event.Kind),
		Time:             event.Time,
		Duration:         event.Duration,
		RuntimeRunID:     event.RunID,
		TurnID:           event.TurnID,
		SessionID:        event.SessionID,
		AppID:            event.AppID,
		UserID:           event.UserID,
		AgentName:        event.AgentName,
		Model:            event.Model,
		Iteration:        event.Iteration,
		Stream:           event.Stream,
		EventAuthor:      event.EventAuthor,
		EventRole:        string(event.EventRole),
		EventCount:       event.EventCount,
		Partial:          event.Partial,
		ToolName:         event.ToolName,
		ToolCallID:       event.ToolCallID,
		FinishReason:     string(event.FinishReason),
		PromptTokens:     event.PromptTokens,
		CompletionTokens: event.CompletionTokens,
		TotalTokens:      event.TotalTokens,
		PartialResponses: event.PartialResponses,
		StoppedEarly:     event.StoppedEarly,
		IsError:          event.IsError,
		Attributes:       safeTraceAttributes(event.Attributes),
	}
	if event.EventID != 0 {
		record.EventID = strconv.FormatInt(event.EventID, 10)
	}
	if event.Kind == adktrace.KindToolCall || event.ToolName != "" || event.ToolCallID != "" {
		toolIndex := event.ToolIndex
		record.ToolIndex = &toolIndex
	}
	if event.Err != nil {
		record.Error = event.Err.Error()
	}
	return record
}

func safeTraceAttributes(attributes map[string]any) map[string]any {
	if len(attributes) == 0 {
		return nil
	}
	safe := make(map[string]any, len(attributes))
	for key, value := range attributes {
		if _, err := json.Marshal(value); err == nil {
			safe[key] = value
		} else {
			safe[key] = fmt.Sprint(value)
		}
	}
	return safe
}

var _ adktrace.Tracer = (*runTracer)(nil)
var _ adktrace.Span = (*runSpan)(nil)
