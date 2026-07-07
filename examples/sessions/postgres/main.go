package main

import (
	"context"
	"log"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	studio "github.com/soasurs/adk-studio"
	"github.com/soasurs/adk-studio/examples/internal/demo"
	"github.com/soasurs/adk/session/database"
)

func main() {
	ctx := context.Background()
	dsn := postgresDSN()
	if dsn == "" {
		log.Fatal("ADK_STUDIO_POSTGRES_DSN or POSTGRES_DSN is required")
	}

	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
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

	app := studio.NewApp(studio.AppConfig{Name: "postgres-session-example", LogLevel: studio.LogLevelInfo})
	app.MustRegisterAgent(demo.NewEchoAgent(
		"postgres_session_agent",
		"Echo agent backed by PostgreSQL session storage.",
	))
	if err := app.UseSessionServiceWithBackend(sessionService, studio.SessionBackendSummary{
		ID:          "postgres",
		Name:        "PostgreSQL database",
		Description: "Shared SQL sessions through ADK session/database and a PostgreSQL *sqlx.DB.",
	}); err != nil {
		log.Fatal(err)
	}

	addr := demo.Addr()
	log.Printf("PostgreSQL session example listening on %s", demo.URL(addr))
	log.Printf("This example does not install a distributed RunLocker; add one for multi-process deployments.")
	if err := studio.Serve(ctx, app, addr); err != nil {
		log.Fatal(err)
	}
}

func postgresDSN() string {
	if dsn := strings.TrimSpace(os.Getenv("ADK_STUDIO_POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	return strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
}
