# Repository Guidelines

## Project Structure & Module Organization

This repository is the Go module `github.com/soasurs/adk-studio` and targets Go
1.26. The root package `studio` contains the embeddable app, HTTP handler,
protocol types, logging, and static asset wiring (`app.go`, `handler.go`,
`protocol.go`, `assets.go`). Go tests live beside the code, such as
`handler_test.go`.

The React workbench lives in `frontend/` and uses Vite, TypeScript, React 19,
Tailwind CSS, Radix UI, and lucide icons. Example hosts live under
`examples/agents/` and `examples/sessions/`. Documentation assets are under
`docs/images/`.

## Build, Test, and Development Commands

- `cd frontend && pnpm install`: install frontend dependencies using the pinned
  pnpm package manager.
- `cd frontend && pnpm build`: type-check and build the Vite UI into
  `frontend/dist`.
- `go test ./...`: run all Go tests. Build the frontend first because the Go
  package embeds `frontend/dist`.
- `go build ./...`: verify all packages and examples compile.
- `go run ./examples/agents/llm`: run an example Studio server on `:18080`.
- `cd frontend && pnpm dev`: start the Vite dev server; it proxies `/api/*` to
  `http://127.0.0.1:18080`.

When running Go commands in agent workflows, use a writable cache, for example
`GOCACHE=/tmp/adk-studio-go-cache go test ./...`.

## Coding Style & Naming Conventions

Format Go code with `gofmt` and keep exported identifiers documented when they
are part of the public embedding API. Prefer explicit, provider-neutral API
types in the root package and keep example-only code inside `examples/`.

Frontend code uses TypeScript and React components in PascalCase
(`Playground.tsx`, `TraceInspector.tsx`). Shared helpers use camelCase file and
function names, such as `formatDisplay.ts`.

## Runtime Protocol Synchronization

Studio merges runner events and concurrent tracer callbacks through one bounded
FIFO feed. Keep response writing serialized, propagate cancellation to Runner,
and wait for Runner shutdown so incomplete-turn rollback finishes. Final span
end records must precede terminal `error` or `done` frames.

When changing ADK event, trace, or failure payloads, update the Go wire types in
`protocol.go` and the matching frontend types and reducers in
`frontend/src/types.ts`, `frontend/src/runEvents.ts`, and
`frontend/src/traceView.ts`. JSON responses omit partial model events but retain
runtime trace ordering; SSE includes partial frames. Typed unknown-tool failures
must not expose tool arguments. Keep 64-bit event IDs as decimal strings in
Studio wire types so browser clients do not lose Snowflake precision.

## Testing Guidelines

Use Go's standard `testing` package for backend behavior. Name tests
`TestXxx` and place them in the same package as the code under test. Add focused
handler tests for HTTP behavior, SSE streaming, and JSON compatibility. There is
no separate frontend test runner configured; use `pnpm build` as the baseline
frontend validation.

Runtime tests should cover trace ordering and host-tracer fan-out, turn
correlation, typed failures, session rollback, and cancellation caused by
client disconnects or SSE write failures. Example tool tests should distinguish
model-visible handled failures (`Result.IsError=true`, nil Go error) from
terminal cancellation or infrastructure errors.

## Commit & Pull Request Guidelines

Use English Angular-style commit subjects, matching existing history:
`feat(ui): refresh studio workbench`, `feat(runs): stream agent events over SSE`.
Use scopes when helpful and mark breaking changes with `!`.

Pull requests should describe the behavior change, list validation commands run,
link related issues when available, and include screenshots for visible UI
changes. Do not commit generated `frontend/dist` output.

## Security & Configuration Tips

Agent examples require `DEEPSEEK_API_KEY`; keep secrets in the environment, not
in source. Session examples may use SQLite or PostgreSQL DSNs; avoid committing
local database files or credentials.
