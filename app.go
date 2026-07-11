package studio

import (
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"

	adkagent "github.com/soasurs/adk/agent"
	"github.com/soasurs/adk/session"
	"github.com/soasurs/adk/trace"
)

type AppConfig struct {
	Name string
	// LogLevel controls the default Studio logger when Logger is nil.
	// The zero value logs at INFO.
	LogLevel LogLevel
	// Logger overrides the default Studio logger. When set, LogLevel is ignored.
	Logger *slog.Logger
	// Tracer receives the same ADK runtime spans that Studio collects for its UI.
	// A nil tracer keeps host-side tracing disabled.
	Tracer trace.Tracer
}

type App struct {
	mu             sync.RWMutex
	name           string
	logger         *slog.Logger
	tracer         trace.Tracer
	agents         map[string]adkagent.Agent
	sessions       session.SessionService
	sessionBackend SessionBackendSummary
}

func NewApp(config AppConfig) *App {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = "adk-studio"
	}
	logger := config.Logger
	if logger == nil {
		logger = NewLogger(nil, config.LogLevel)
	}
	return &App{
		name:   name,
		logger: logger,
		tracer: config.Tracer,
		agents: make(map[string]adkagent.Agent),
	}
}

func (a *App) tracerForRun() trace.Tracer {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tracer
}

func (a *App) Name() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.name
}

func (a *App) loggerForRun() *slog.Logger {
	if a == nil || a.logger == nil {
		return slog.Default()
	}
	return a.logger
}

func (a *App) RegisterAgent(agent adkagent.Agent) error {
	if isNil(agent) {
		return fmt.Errorf("studio: agent must not be nil")
	}
	name := strings.TrimSpace(agent.Name())
	if name == "" {
		return fmt.Errorf("studio: agent name must not be empty")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.agents[name]; exists {
		return fmt.Errorf("studio: agent %q is already registered", name)
	}
	a.agents[name] = agent
	return nil
}

func (a *App) MustRegisterAgent(agent adkagent.Agent) {
	if err := a.RegisterAgent(agent); err != nil {
		panic(err)
	}
}

func (a *App) UseSessionService(service session.SessionService) error {
	return a.UseSessionServiceWithBackend(service, inferSessionBackend(service))
}

func (a *App) UseSessionServiceWithBackend(service session.SessionService, backend SessionBackendSummary) error {
	if isNil(service) {
		return fmt.Errorf("studio: session service must not be nil")
	}
	backend = normalizeSessionBackend(backend)
	if backend.ID == "" {
		return fmt.Errorf("studio: session backend id must not be empty")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions = service
	a.sessionBackend = backend
	return nil
}

func (a *App) agentSummaries() []AgentSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()

	agents := make([]AgentSummary, 0, len(a.agents))
	for name, agent := range a.agents {
		agents = append(agents, AgentSummary{
			ID:          name,
			Name:        name,
			Description: agent.Description(),
		})
	}
	slices.SortFunc(agents, func(a, b AgentSummary) int {
		return strings.Compare(a.Name, b.Name)
	})
	return agents
}

func (a *App) agentSummary(id string) (AgentSummary, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	agent, ok := a.agents[id]
	if !ok {
		return AgentSummary{}, false
	}
	return AgentSummary{
		ID:          id,
		Name:        id,
		Description: agent.Description(),
	}, true
}

func (a *App) agent(id string) (adkagent.Agent, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	agent, ok := a.agents[id]
	return agent, ok
}

func (a *App) sessionService() session.SessionService {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sessions
}

func (a *App) hasSessionService() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sessions != nil
}

func (a *App) sessionBackendSummary() *SessionBackendSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.sessions == nil || a.sessionBackend.ID == "" {
		return nil
	}
	backend := a.sessionBackend
	return &backend
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func normalizeSessionBackend(backend SessionBackendSummary) SessionBackendSummary {
	backend.ID = strings.TrimSpace(backend.ID)
	backend.Name = strings.TrimSpace(backend.Name)
	backend.Description = strings.TrimSpace(backend.Description)
	if backend.ID == "" {
		backend.ID = backend.Name
	}
	if backend.Name == "" {
		backend.Name = backend.ID
	}
	return backend
}

func inferSessionBackend(service session.SessionService) SessionBackendSummary {
	if isNil(service) {
		return SessionBackendSummary{}
	}
	serviceType := reflect.TypeOf(service)
	for serviceType.Kind() == reflect.Pointer {
		serviceType = serviceType.Elem()
	}

	switch serviceType.PkgPath() {
	case "github.com/soasurs/adk/session/memory":
		return SessionBackendSummary{
			ID:          "memory",
			Name:        "Memory",
			Description: "Process-local in-memory sessions for examples, tests, and local development.",
		}
	case "github.com/soasurs/adk/session/database":
		return SessionBackendSummary{
			ID:          "database",
			Name:        "SQL database",
			Description: "SQL-backed sessions through session/database; the host app owns the database driver and connection.",
		}
	default:
		name := serviceType.Name()
		if name == "" {
			name = serviceType.String()
		}
		return SessionBackendSummary{
			ID:   serviceType.PkgPath() + "." + name,
			Name: name,
		}
	}
}
