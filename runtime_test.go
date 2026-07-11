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
	"sync/atomic"
	"testing"
	"time"

	"github.com/soasurs/adk/model"
	"github.com/soasurs/adk/session"
	sessionevent "github.com/soasurs/adk/session/event"
	"github.com/soasurs/adk/session/memory"
	adktrace "github.com/soasurs/adk/trace"
)

type recordingHostTracer struct {
	mu            sync.Mutex
	starts        int
	ends          int
	nestedContext int
}

type hostTraceContextKey struct{}

func (t *recordingHostTracer) Start(ctx context.Context, _ adktrace.Event) (context.Context, adktrace.Span) {
	t.mu.Lock()
	t.starts++
	if ctx.Value(hostTraceContextKey{}) == true {
		t.nestedContext++
	}
	t.mu.Unlock()
	return context.WithValue(ctx, hostTraceContextKey{}, true), &recordingHostSpan{tracer: t}
}

type recordingHostSpan struct {
	tracer *recordingHostTracer
}

func (*recordingHostSpan) AddEvent(context.Context, adktrace.Event) {}

func (s *recordingHostSpan) End(context.Context, adktrace.Event) {
	s.tracer.mu.Lock()
	s.tracer.ends++
	s.tracer.mu.Unlock()
}

func (t *recordingHostTracer) counts() (starts, ends, nested int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.starts, t.ends, t.nestedContext
}

func TestHandlerCollectsRuntimeTracesAndFansOutHostTracer(t *testing.T) {
	host := &recordingHostTracer{}
	app := NewApp(AppConfig{Name: "test", Tracer: host})
	app.MustRegisterAgent(testAgent{})
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		t.Fatal(err)
	}

	response := runJSONRequest(t, app, "test_agent", "session-trace")
	if len(response.Events) < 3 {
		t.Fatalf("events = %#v, want trace, event, and done frames", response.Events)
	}
	if response.Events[len(response.Events)-1].Type != "done" {
		t.Fatalf("last frame = %q, want done", response.Events[len(response.Events)-1].Type)
	}
	lastTrace := response.Events[len(response.Events)-2]
	if lastTrace.Type != "trace" || lastTrace.Trace == nil || lastTrace.Trace.Phase != "end" || lastTrace.Trace.Kind != string(adktrace.KindRunnerRun) {
		t.Fatalf("frame before done = %#v, want runner end trace", lastTrace)
	}

	eventFrame := findRunFrame(response.Events, "event")
	if eventFrame == nil || eventFrame.Event == nil || eventFrame.Event.TurnID == "" {
		t.Fatalf("event turn correlation was missing: %#v", eventFrame)
	}
	var runnerTrace *RunTraceRecord
	for _, frame := range response.Events {
		if frame.Trace != nil && frame.Trace.Kind == string(adktrace.KindRunnerRun) {
			runnerTrace = frame.Trace
			break
		}
	}
	if runnerTrace == nil || runnerTrace.TurnID != eventFrame.Event.TurnID {
		t.Fatalf("runner trace turn = %#v, event turn = %q", runnerTrace, eventFrame.Event.TurnID)
	}

	starts, ends, nested := host.counts()
	if starts == 0 || ends != starts {
		t.Fatalf("host spans starts=%d ends=%d", starts, ends)
	}
	if nested == 0 {
		t.Fatal("host tracer context was not propagated to nested spans")
	}
}

func TestTraceRecordPreservesZeroToolIndex(t *testing.T) {
	record := traceRecord("start", adktrace.Event{
		Kind:       adktrace.KindToolCall,
		Time:       time.Now(),
		ToolName:   "lookup",
		ToolCallID: "call-1",
		ToolIndex:  0,
	})
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"tool_index":0`)) {
		t.Fatalf("trace JSON = %s, want tool_index=0", data)
	}
}

func TestTraceRecordPreservesLargeEventID(t *testing.T) {
	record := traceRecord("end", adktrace.Event{
		Kind:    adktrace.KindEventPersist,
		Time:    time.Now(),
		EventID: 9007199254740993,
	})
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"event_id":"9007199254740993"`)) {
		t.Fatalf("trace JSON = %s, want exact decimal-string event ID", data)
	}
}

func TestRunEventJSONIncludesTurnAndUsageDetails(t *testing.T) {
	frame := RunStreamEvent{
		Type:  "event",
		RunID: "studio-run",
		Event: &model.Event{
			TurnID: "turn-1",
			Usage: &model.TokenUsage{
				PromptTokens: 5,
				Details: &model.TokenUsageDetails{
					CachedPromptTokens:       4,
					ReasoningTokens:          3,
					AcceptedPredictionTokens: 2,
				},
			},
		},
	}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"TurnID":"turn-1"`, `"cached_prompt_tokens":4`, `"reasoning_tokens":3`, `"accepted_prediction_tokens":2`} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("event JSON = %s, want %s", data, want)
		}
	}
}

type eventThenFailAgent struct{}

func (eventThenFailAgent) Name() string        { return "event_then_fail" }
func (eventThenFailAgent) Description() string { return "Yields an event and then fails." }
func (eventThenFailAgent) Run(context.Context, []model.Event) iter.Seq2[*model.Event, error] {
	return func(yield func(*model.Event, error) bool) {
		if !yield(&model.Event{
			Author:  "event_then_fail",
			Content: model.Content{Role: model.RoleAssistant, Content: "attempted response"},
		}, nil) {
			return
		}
		yield(nil, errors.New("terminal provider failure"))
	}
}

func TestHandlerFailedRunRollsBackTurnAndKeepsAttemptFeed(t *testing.T) {
	sessions := memory.NewMemorySessionService()
	app := NewApp(AppConfig{Name: "test"})
	app.MustRegisterAgent(eventThenFailAgent{})
	if err := app.UseSessionService(sessions); err != nil {
		t.Fatal(err)
	}

	response := runJSONRequestWithStatus(t, app, "event_then_fail", "session-failed", http.StatusInternalServerError)
	if findRunFrame(response.Events, "event") == nil || findRunFrame(response.Events, "trace") == nil {
		t.Fatalf("failed attempt feed was incomplete: %#v", response.Events)
	}
	errorFrame := findRunFrame(response.Events, "error")
	if errorFrame == nil || errorFrame.Failure == nil || errorFrame.Failure.Code != "run_failed" {
		t.Fatalf("failure frame = %#v", errorFrame)
	}
	if response.Events[len(response.Events)-2].Trace == nil || response.Events[len(response.Events)-1].Type != "error" {
		t.Fatalf("final span must precede error: %#v", response.Events[len(response.Events)-2:])
	}

	assertSessionEmpty(t, sessions, "session-failed")
}

type invocationCountingAgent struct {
	called atomic.Bool
}

func (*invocationCountingAgent) Name() string        { return "counting" }
func (*invocationCountingAgent) Description() string { return "Counts invocations." }
func (a *invocationCountingAgent) Run(context.Context, []model.Event) iter.Seq2[*model.Event, error] {
	a.called.Store(true)
	return func(func(*model.Event, error) bool) {}
}

func TestHandlerReturnsTypedUnknownToolExecutionWithoutRunningAgent(t *testing.T) {
	ctx := t.Context()
	sessions := memory.NewMemorySessionService()
	_, err := sessions.CreateSession(ctx, session.CreateSessionRequest{SessionID: "session-unknown", AppID: "test", UserID: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	sess, err := sessions.GetSession(ctx, "session-unknown")
	if err != nil {
		t.Fatal(err)
	}
	err = sess.CreateEvent(ctx, sessionevent.FromModel(model.Event{
		ID:        100,
		SessionID: "session-unknown",
		TurnID:    "source-turn",
		Author:    "assistant",
		Content: model.Content{
			Role: model.RoleAssistant,
			ToolCalls: []model.ToolCall{{
				ID:        "call-unknown",
				Name:      "dangerous_tool",
				Arguments: []byte(`{"secret":"must-not-leak"}`),
			}},
		},
		CreatedAt: 1,
		UpdatedAt: 1,
	}))
	if err != nil {
		t.Fatal(err)
	}

	agent := &invocationCountingAgent{}
	app := NewApp(AppConfig{Name: "test"})
	app.MustRegisterAgent(agent)
	if err := app.UseSessionService(sessions); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := newRunRequest("counting", "session-unknown")
	NewHandler(app).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if agent.called.Load() {
		t.Fatal("agent ran despite unresolved tool execution")
	}
	var response RunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	errorFrame := findRunFrame(response.Events, "error")
	if errorFrame == nil || errorFrame.Failure == nil {
		t.Fatalf("typed failure was missing: %#v", response.Events)
	}
	failure := errorFrame.Failure
	if failure.Code != "tool_execution_unknown" || failure.SourceTurnID != "source-turn" || failure.SourceEventID != "100" {
		t.Fatalf("failure = %#v", failure)
	}
	if len(failure.UnresolvedTools) != 1 || failure.UnresolvedTools[0].ID != "call-unknown" || failure.UnresolvedTools[0].Name != "dangerous_tool" {
		t.Fatalf("unresolved tools = %#v", failure.UnresolvedTools)
	}
	if strings.Contains(recorder.Body.String(), "must-not-leak") || strings.Contains(recorder.Body.String(), `"arguments"`) {
		t.Fatalf("failure leaked tool arguments: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"source_event_id":"100"`) {
		t.Fatalf("failure did not preserve source event ID as a string: %s", recorder.Body.String())
	}
}

type cancellationReportingAgent struct {
	started chan struct{}
}

func (cancellationReportingAgent) Name() string        { return "cancellation_reporting" }
func (cancellationReportingAgent) Description() string { return "Reports request cancellation." }
func (a cancellationReportingAgent) Run(ctx context.Context, _ []model.Event) iter.Seq2[*model.Event, error] {
	return func(yield func(*model.Event, error) bool) {
		close(a.started)
		<-ctx.Done()
		yield(nil, ctx.Err())
	}
}

func TestHandlerJSONCancellationReturnsFailure(t *testing.T) {
	sessions := memory.NewMemorySessionService()
	agent := cancellationReportingAgent{started: make(chan struct{})}
	app := NewApp(AppConfig{Name: "test"})
	app.MustRegisterAgent(agent)
	if err := app.UseSessionService(sessions); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	recorder := httptest.NewRecorder()
	request := newRunRequest(agent.Name(), "session-json-cancel").WithContext(ctx)
	done := make(chan struct{})
	go func() {
		NewHandler(app).ServeHTTP(recorder, request)
		close(done)
	}()

	select {
	case <-agent.started:
	case <-time.After(time.Second):
		t.Fatal("agent did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after request cancellation")
	}

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	var response RunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != context.Canceled.Error() {
		t.Fatalf("error = %q, want %q", response.Error, context.Canceled)
	}
	errorFrame := findRunFrame(response.Events, "error")
	if errorFrame == nil || errorFrame.Failure == nil || errorFrame.Failure.Code != "run_failed" {
		t.Fatalf("cancellation failure frame = %#v", errorFrame)
	}
	if findRunFrame(response.Events, "done") != nil {
		t.Fatalf("canceled response contained done frame: %#v", response.Events)
	}
	assertSessionEmpty(t, sessions, "session-json-cancel")
}

func TestHandlerClientCancellationRollsBackStreamingTurn(t *testing.T) {
	sessions := memory.NewMemorySessionService()
	agent := blockingStreamingAgent{release: make(chan struct{})}
	app := NewApp(AppConfig{Name: "test"})
	app.MustRegisterAgent(agent)
	if err := app.UseSessionService(sessions); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	recorder := newFlushRecorder()
	request := newRunRequest("blocking_streaming_agent", "session-cancel").WithContext(ctx)
	request.Header.Set("Accept", "text/event-stream")
	done := make(chan struct{})
	go func() {
		NewHandler(app).ServeHTTP(recorder, request)
		close(done)
	}()
	waitForStreamText(t, recorder, `"type":"partial"`)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not wait for canceled runner")
	}
	assertSessionEmpty(t, sessions, "session-cancel")
}

type flushFailureRecorder struct {
	*flushRecorder
}

func (r *flushFailureRecorder) FlushError() error {
	_, _, body := r.snapshot()
	if strings.Contains(body, `"type":"partial"`) {
		return errors.New("client write failed")
	}
	r.Flush()
	return nil
}

func TestHandlerSSEFlushFailureCancelsAndRollsBackTurn(t *testing.T) {
	sessions := memory.NewMemorySessionService()
	agent := blockingStreamingAgent{release: make(chan struct{})}
	app := NewApp(AppConfig{Name: "test"})
	app.MustRegisterAgent(agent)
	if err := app.UseSessionService(sessions); err != nil {
		t.Fatal(err)
	}

	recorder := &flushFailureRecorder{flushRecorder: newFlushRecorder()}
	request := newRunRequest("blocking_streaming_agent", "session-write-fail")
	request.Header.Set("Accept", "text/event-stream")
	done := make(chan struct{})
	go func() {
		NewHandler(app).ServeHTTP(recorder, request)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not cancel runner after SSE flush failure")
	}
	assertSessionEmpty(t, sessions, "session-write-fail")
}

func runJSONRequest(t *testing.T, app *App, agentID, sessionID string) RunResponse {
	t.Helper()
	return runJSONRequestWithStatus(t, app, agentID, sessionID, http.StatusOK)
}

func runJSONRequestWithStatus(t *testing.T, app *App, agentID, sessionID string, status int) RunResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	NewHandler(app).ServeHTTP(recorder, newRunRequest(agentID, sessionID))
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, status, recorder.Body.String())
	}
	var response RunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func newRunRequest(agentID, sessionID string) *http.Request {
	body := strings.NewReader(`{"agent_id":"` + agentID + `","app_name":"test","user_id":"dev","session_id":"` + sessionID + `","input":{"content":"hello"}}`)
	return httptest.NewRequest(http.MethodPost, "/api/runs", body)
}

func assertSessionEmpty(t *testing.T, sessions session.SessionService, sessionID string) {
	t.Helper()
	sess, err := sessions.GetSession(t.Context(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if sess == nil {
		t.Fatalf("session %q was not created", sessionID)
	}
	events, err := sess.ListEvents(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("session events = %#v, want rollback to empty", events)
	}
}

func waitForStreamText(t *testing.T, recorder *flushRecorder, text string) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case <-recorder.flushes:
			_, _, body := recorder.snapshot()
			if strings.Contains(body, text) {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q", text)
		}
	}
}

var _ adktrace.Tracer = (*recordingHostTracer)(nil)
