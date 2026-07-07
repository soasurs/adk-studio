package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	studio "github.com/soasurs/adk-studio"
	"github.com/soasurs/adk-studio/examples/internal/demo"
	"github.com/soasurs/adk/session/database"
)

func main() {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("ADK_STUDIO_SQLITE_DSN"))
	if dsn == "" {
		dsn = "adk-studio-sessions.sqlite3"
	}

	db, err := sqlx.Connect("sqlite3", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := database.InitSchema(ctx, db); err != nil {
		log.Fatal(err)
	}
	sessionService, err := database.NewDatabaseSessionService(db)
	if err != nil {
		log.Fatal(err)
	}

	app := studio.NewApp(studio.AppConfig{Name: "sqlite-session-example", LogLevel: studio.LogLevelInfo})
	app.MustRegisterAgent(demo.NewEchoAgent(
		"sqlite_session_agent",
		"Echo agent backed by SQLite session storage.",
	))
	if err := app.UseSessionServiceWithBackend(sessionService, studio.SessionBackendSummary{
		ID:          "sqlite",
		Name:        "SQLite database",
		Description: "Persistent local sessions through ADK session/database and a SQLite *sqlx.DB.",
	}); err != nil {
		log.Fatal(err)
	}

	addr := demo.Addr()
	log.Printf("SQLite session example listening on %s with DSN %q", demo.URL(addr), dsn)
	log.Printf("Try multiple turns, restart the process, then reuse the same Session ID.")
	if err := studio.Serve(ctx, app, addr); err != nil {
		log.Fatal(err)
	}
}
