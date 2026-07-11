package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/soasurs/adk/model"
	"github.com/soasurs/adk/session/memory"
)

type testAgent struct{}
type failingAgent struct{}
type partialThenCompleteAgent struct{}
type blockingStreamingAgent struct {
	release chan struct{}
}

func (testAgent) Name() string {
	return "test_agent"
}

func (testAgent) Description() string {
	return "Agent used by Studio handler tests."
}

func (testAgent) Run(ctx context.Context, events []model.Event) iter.Seq2[*model.Event, error] {
	return func(yield func(*model.Event, error) bool) {
		latest := ""
		for i := len(events) - 1; i >= 0; i-- {
			if events[i].Content.Role == model.RoleUser {
				latest = events[i].Content.Content
				break
			}
		}
		yield(&model.Event{
			Author: testAgent{}.Name(),
			Content: model.Content{
				Role:    model.RoleAssistant,
				Content: "Echo: " + latest,
			},
		}, nil)
	}
}

func (failingAgent) Name() string {
	return "failing_agent"
}

func (failingAgent) Description() string {
	return "Always fails."
}

func (failingAgent) Run(ctx context.Context, events []model.Event) iter.Seq2[*model.Event, error] {
	return func(yield func(*model.Event, error) bool) {
		yield(nil, errors.New("provider rejected history"))
	}
}

func (partialThenCompleteAgent) Name() string {
	return "partial_then_complete_agent"
}

func (partialThenCompleteAgent) Description() string {
	return "Yields one partial event and one complete event."
}

func (partialThenCompleteAgent) Run(ctx context.Context, events []model.Event) iter.Seq2[*model.Event, error] {
	return func(yield func(*model.Event, error) bool) {
		if !yield(&model.Event{
			Author: partialThenCompleteAgent{}.Name(),
			Content: model.Content{
				Role:    model.RoleAssistant,
				Content: "partial",
			},
			Partial: true,
		}, nil) {
			return
		}
		yield(&model.Event{
			Author: partialThenCompleteAgent{}.Name(),
			Content: model.Content{
				Role:    model.RoleAssistant,
				Content: "complete",
			},
		}, nil)
	}
}

func (blockingStreamingAgent) Name() string {
	return "blocking_streaming_agent"
}

func (blockingStreamingAgent) Description() string {
	return "Yields one partial event, then blocks until released."
}

func (a blockingStreamingAgent) Run(ctx context.Context, events []model.Event) iter.Seq2[*model.Event, error] {
	return func(yield func(*model.Event, error) bool) {
		if !yield(&model.Event{
			Author: a.Name(),
			Content: model.Content{
				Role:    model.RoleAssistant,
				Content: "first",
			},
			Partial: true,
		}, nil) {
			return
		}
		select {
		case <-ctx.Done():
			yield(nil, ctx.Err())
			return
		case <-a.release:
		}
		yield(&model.Event{
			Author: a.Name(),
			Content: model.Content{
				Role:    model.RoleAssistant,
				Content: "first second",
			},
		}, nil)
	}
}

type flushRecorder struct {
	mu      sync.Mutex
	header  http.Header
	code    int
	body    bytes.Buffer
	flushes chan struct{}
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{
		header:  make(http.Header),
		flushes: make(chan struct{}, 10),
	}
}

func (r *flushRecorder) Header() http.Header {
	return r.header
}

func (r *flushRecorder) WriteHeader(statusCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.code == 0 {
		r.code = statusCode
	}
}

func (r *flushRecorder) Write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.body.Write(data)
}

func (r *flushRecorder) Flush() {
	select {
	case r.flushes <- struct{}{}:
	default:
	}
}

func (r *flushRecorder) snapshot() (int, string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.code, r.header.Get("Content-Type"), r.body.String()
}

func TestHandlerListsRegisteredAgents(t *testing.T) {
	app := NewApp(AppConfig{Name: "test"})
	app.MustRegisterAgent(testAgent{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	NewHandler(app).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Agents []AgentSummary `json:"agents"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(response.Agents) != 1 {
		t.Fatalf("agents length = %d, want 1", len(response.Agents))
	}
	if response.Agents[0].ID != "test_agent" {
		t.Fatalf("agent ID = %q, want test_agent", response.Agents[0].ID)
	}
}

func TestHandlerReturnsSessionBackend(t *testing.T) {
	app := NewApp(AppConfig{Name: "test"})
	if err := app.UseSessionServiceWithBackend(memory.NewMemorySessionService(), SessionBackendSummary{
		ID:          "memory",
		Name:        "Memory",
		Description: "Process-local sessions.",
	}); err != nil {
		t.Fatalf("use session service: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/app", nil)
	NewHandler(app).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response AppSummary
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !response.HasSessionService {
		t.Fatal("has session service = false, want true")
	}
	if response.SessionBackend == nil {
		t.Fatal("session backend = nil, want memory")
	}
	if response.SessionBackend.ID != "memory" {
		t.Fatalf("session backend ID = %q, want memory", response.SessionBackend.ID)
	}
}

func TestHandlerServesEmbeddedUI(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	NewHandler(NewApp(AppConfig{Name: "test"})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `<div id="root">`) {
		t.Fatalf("response did not look like embedded UI index")
	}
}

func TestHandlerRunsRegisteredAgent(t *testing.T) {
	app := NewApp(AppConfig{Name: "test"})
	app.MustRegisterAgent(testAgent{})
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		t.Fatalf("use session service: %v", err)
	}

	body := strings.NewReader(`{
		"agent_id": "test_agent",
		"app_name": "test",
		"user_id": "dev",
		"session_id": "session-1",
		"input": {
			"content": "hello"
		}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/runs", body)
	NewHandler(app).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response RunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.SessionID != "session-1" {
		t.Fatalf("session ID = %q, want session-1", response.SessionID)
	}
	eventFrame := findRunFrame(response.Events, "event")
	if eventFrame == nil || eventFrame.Event == nil {
		t.Fatalf("event was nil")
	}
	if eventFrame.Event.Content.Content != "Echo: hello" {
		t.Fatalf("event content = %q, want Echo: hello", eventFrame.Event.Content.Content)
	}
	if findRunFrame(response.Events, "trace") == nil {
		t.Fatal("runtime trace frame was missing")
	}
}

func TestHandlerJSONRunOmitsPartialEvents(t *testing.T) {
	app := NewApp(AppConfig{Name: "test"})
	app.MustRegisterAgent(partialThenCompleteAgent{})
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		t.Fatalf("use session service: %v", err)
	}

	body := strings.NewReader(`{
		"agent_id": "partial_then_complete_agent",
		"app_name": "test",
		"user_id": "dev",
		"session_id": "session-1",
		"input": {
			"content": "hello"
		}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/runs", body)
	NewHandler(app).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response RunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	for _, frame := range response.Events {
		if frame.Type == "partial" || frame.Event != nil && frame.Event.Partial {
			t.Fatalf("JSON response contained partial frame: %#v", frame)
		}
	}
	eventFrame := findRunFrame(response.Events, "event")
	if eventFrame == nil {
		t.Fatal("complete event frame was missing")
	}
	event := eventFrame.Event
	if event == nil {
		t.Fatal("event was nil")
	}
	if event.Partial {
		t.Fatalf("event partial = true, want false")
	}
	if event.Content.Content != "complete" {
		t.Fatalf("event content = %q, want complete", event.Content.Content)
	}
}

func TestHandlerStreamsRunEventsWhenRequested(t *testing.T) {
	app := NewApp(AppConfig{Name: "test"})
	agent := blockingStreamingAgent{release: make(chan struct{})}
	released := false
	defer func() {
		if !released {
			close(agent.release)
		}
	}()
	app.MustRegisterAgent(agent)
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		t.Fatalf("use session service: %v", err)
	}

	body := strings.NewReader(`{
		"agent_id": "blocking_streaming_agent",
		"app_name": "test",
		"user_id": "dev",
		"session_id": "session-1",
		"input": {
			"content": "hello"
		}
	}`)
	recorder := newFlushRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/runs", body)
	request.Header.Set("Accept", "text/event-stream")

	done := make(chan struct{})
	go func() {
		NewHandler(app).ServeHTTP(recorder, request)
		close(done)
	}()

	deadline := time.After(time.Second)
	for {
		select {
		case <-recorder.flushes:
			_, _, streamedBody := recorder.snapshot()
			if strings.Contains(streamedBody, "event: partial\n") {
				goto partialReceived
			}
		case <-deadline:
			t.Fatal("timed out waiting for streamed partial")
		}
	}

partialReceived:

	status, contentType, streamedBody := recorder.snapshot()
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if contentType != "text/event-stream" {
		t.Fatalf("content type = %q, want text/event-stream", contentType)
	}
	if !strings.Contains(streamedBody, "event: partial\n") {
		t.Fatalf("streamed body did not contain partial frame: %q", streamedBody)
	}
	if !strings.Contains(streamedBody, `"Partial":true`) {
		t.Fatalf("streamed body did not contain partial event: %q", streamedBody)
	}

	select {
	case <-done:
		t.Fatal("handler completed before agent was released")
	default:
	}

	close(agent.release)
	released = true
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for streamed run to complete")
	}

	_, _, finalBody := recorder.snapshot()
	if !strings.Contains(finalBody, "event: done\n") {
		t.Fatalf("streamed body did not contain done frame: %q", finalBody)
	}
	if !strings.Contains(finalBody, "event: event\n") {
		t.Fatalf("streamed body did not contain complete event frame: %q", finalBody)
	}
	if !strings.Contains(finalBody, "first second") {
		t.Fatalf("streamed body did not contain complete event: %q", finalBody)
	}
	endIndex := strings.LastIndex(finalBody, `"phase":"end","kind":"adk.runner.run"`)
	doneIndex := strings.LastIndex(finalBody, "event: done\n")
	if endIndex < 0 || doneIndex < 0 || endIndex > doneIndex {
		t.Fatalf("runner end trace must precede done frame: %q", finalBody)
	}
}

func TestHandlerLogsRunEventsAtInfo(t *testing.T) {
	var logs bytes.Buffer
	app := NewApp(AppConfig{
		Name:   "test",
		Logger: NewLogger(&logs, LogLevelInfo),
	})
	app.MustRegisterAgent(testAgent{})
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		t.Fatalf("use session service: %v", err)
	}

	body := strings.NewReader(`{
		"agent_id": "test_agent",
		"app_name": "test",
		"user_id": "dev",
		"session_id": "session-1",
		"input": {
			"content": "hello"
		}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/runs", body)
	NewHandler(app).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	output := logs.String()
	if !strings.Contains(output, "level=INFO") {
		t.Fatalf("expected INFO log, got %q", output)
	}
	if !strings.Contains(output, `msg="adk studio event"`) {
		t.Fatalf("expected event log, got %q", output)
	}
	if !strings.Contains(output, "author=test_agent") {
		t.Fatalf("expected event author in log, got %q", output)
	}
	if !strings.Contains(output, "Echo: hello") {
		t.Fatalf("expected serialized event content in log, got %q", output)
	}
	if !strings.Contains(output, "turn_id=") {
		t.Fatalf("expected turn_id in log, got %q", output)
	}
}

func TestHandlerSuppressesRunEventsAboveInfo(t *testing.T) {
	var logs bytes.Buffer
	app := NewApp(AppConfig{
		Name:   "test",
		Logger: NewLogger(&logs, LogLevelWarn),
	})
	app.MustRegisterAgent(testAgent{})
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		t.Fatalf("use session service: %v", err)
	}

	body := strings.NewReader(`{
		"agent_id": "test_agent",
		"app_name": "test",
		"user_id": "dev",
		"session_id": "session-1",
		"input": {
			"content": "hello"
		}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/runs", body)
	NewHandler(app).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if output := logs.String(); strings.Contains(output, "adk studio event") {
		t.Fatalf("expected WARN logger to suppress event log, got %q", output)
	}
}

func TestHandlerRunErrorIncludesTopLevelMessage(t *testing.T) {
	app := NewApp(AppConfig{Name: "test"})
	app.MustRegisterAgent(failingAgent{})
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		t.Fatalf("use session service: %v", err)
	}

	body := strings.NewReader(`{
		"agent_id": "failing_agent",
		"app_name": "test",
		"user_id": "dev",
		"session_id": "session-1",
		"input": {
			"content": "hello"
		}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/runs", body)
	NewHandler(app).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	var response RunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Error != "provider rejected history" {
		t.Fatalf("error = %q, want provider rejected history", response.Error)
	}
	errorFrame := findRunFrame(response.Events, "error")
	if errorFrame == nil || errorFrame.Error != "provider rejected history" {
		t.Fatalf("expected error event, got %#v", response.Events)
	}
	if errorFrame.Failure == nil || errorFrame.Failure.Code != "run_failed" {
		t.Fatalf("typed failure = %#v, want run_failed", errorFrame.Failure)
	}
}

func findRunFrame(events []RunStreamEvent, eventType string) *RunStreamEvent {
	for i := range events {
		if events[i].Type == eventType {
			return &events[i]
		}
	}
	return nil
}
