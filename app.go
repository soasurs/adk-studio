package studio

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	adkagent "github.com/soasurs/adk/agent"
	"github.com/soasurs/adk/session"
)

type AppConfig struct {
	Name string
}

type App struct {
	mu       sync.RWMutex
	name     string
	agents   map[string]adkagent.Agent
	sessions session.SessionService
}

func NewApp(config AppConfig) *App {
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = "adk-studio"
	}
	return &App{
		name:   name,
		agents: make(map[string]adkagent.Agent),
	}
}

func (a *App) Name() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.name
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
	if isNil(service) {
		return fmt.Errorf("studio: session service must not be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions = service
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
