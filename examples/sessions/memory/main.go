package main

import (
	"context"
	"log"

	studio "github.com/soasurs/adk-studio"
	"github.com/soasurs/adk-studio/examples/internal/demo"
	"github.com/soasurs/adk/session/memory"
)

func main() {
	ctx := context.Background()

	app := studio.NewApp(studio.AppConfig{Name: "memory-session-example", LogLevel: studio.LogLevelInfo})
	app.MustRegisterAgent(demo.NewEchoAgent(
		"memory_session_agent",
		"Echo agent backed by the in-memory session service.",
	))
	if err := app.UseSessionServiceWithBackend(memory.NewMemorySessionService(), studio.SessionBackendSummary{
		ID:          "memory",
		Name:        "Memory",
		Description: "Process-local in-memory sessions for examples, tests, and local development.",
	}); err != nil {
		log.Fatal(err)
	}

	addr := demo.Addr()
	log.Printf("Memory session example listening on %s", demo.URL(addr))
	log.Printf("Try multiple turns in the same Session ID to inspect stored history.")
	if err := studio.Serve(ctx, app, addr); err != nil {
		log.Fatal(err)
	}
}
