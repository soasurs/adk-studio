# ADK Studio

Development studio for [`github.com/soasurs/adk`](https://github.com/soasurs/adk).

[中文文档](./README.zh-CN.md)

ADK Studio is an embeddable React workbench for developing, testing, and
observing ADK agents. Users define agents in normal Go code, register them with
Studio, and serve the Studio UI from the same process. Studio does not
dynamically load arbitrary Go source.

## Architecture

This repository is split into three parts:

- root Go package `studio`: embeddable Studio app, HTTP handler, and static UI
  serving.
- `frontend`: React + Vite frontend. No Next.js.
- `examples`: runnable examples that show how a user project hosts Studio.

The intended runtime boundary is:

```text
React Studio UI
        ↓ same HTTP server
studio.NewHandler(app)
        ↓
github.com/soasurs/adk Runner + Agent + Tools + Session
```

`frontend/dist` is generated build output and is intentionally ignored by Git.
Run the frontend build before compiling Go code that embeds the UI.

## Current Scope

This is still a small Studio skeleton, but the main loop is functional:

- register one or more ADK agents in a `studio.App`.
- provide a session service for multi-turn runs.
- discover registered agents through `/api/agents`.
- run the selected agent through `POST /api/runs`.
- inspect returned ADK events in the trace panel.
- display assistant messages, reasoning content, tool calls, and tool results
  as separate UI entries.
- optionally stream live ADK events over SSE when the Studio UI runs an agent.
- use a fixed-height React workbench with sidebar controls, playground, trace
  inspector, and configurable send shortcut.

The run API keeps the completed JSON response for ordinary clients. Clients that
send `Accept: text/event-stream` receive live SSE frames as each ADK event is
produced.

## Build

Install and build the frontend first:

```bash
cd frontend
pnpm install
pnpm build
cd ..
```

Then build or test the Go package:

```bash
go test ./...
go build ./...
```

The Go package embeds `frontend/dist` with `go:embed`, so a fresh checkout needs
the frontend build before `go test`, `go build`, or `go run` can compile the
handler.

## Run the Example

The embedded example registers a DeepSeek-backed `llmagent` with local fixture
tools and Exa MCP search tools.

```bash
export DEEPSEEK_API_KEY=...
# Optional:
export DEEPSEEK_MODEL=...
export EXA_API_KEY=...

go run ./examples/embedded
```

Open [http://127.0.0.1:18080](http://127.0.0.1:18080).

Useful prompts:

```text
帮我检查 Alex 的订单，看看为什么发货延迟，并给一个处理建议。
用 Exa 搜索 github.com/soasurs/adk 的相关信息，并总结来源。
```

The local tool prompt is designed to force multiple tool-call rounds:
`lookup_customer` → `inspect_order` → `recommend_resolution`.

## Frontend Development

Run the Go example on `:18080`, then start Vite:

```bash
cd frontend
pnpm dev
```

The Vite dev server proxies `/api/*` requests to
`http://127.0.0.1:18080`.

Production assets are generated with:

```bash
cd frontend
pnpm build
```

Generated files under `frontend/dist` should not be committed.

## Embedding Studio

Integration in a user project should look like this:

```go
package main

import (
    "context"
    "log"

    studio "github.com/soasurs/adk-studio"
    "github.com/soasurs/adk/session/memory"
)

func main() {
    ctx := context.Background()

    app := studio.NewApp(studio.AppConfig{
        Name:     "demo",
        LogLevel: studio.LogLevelInfo,
    })
    app.MustRegisterAgent(myAgent)
    if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
        log.Fatal(err)
    }

    if err := studio.Serve(ctx, app, ":18080"); err != nil {
        log.Fatal(err)
    }
}
```

For more control, mount the handler yourself:

```go
http.ListenAndServe(":18080", studio.NewHandler(app))
```

Studio logs to `stderr` with Go's `log/slog` text handler at INFO level by
default. Every ADK event returned by a run is logged at INFO. Use
`LogLevelDebug`, `LogLevelWarn`, `LogLevelError`, or `LogLevelOff`, or pass a
custom `*slog.Logger` in `AppConfig.Logger` when embedding Studio.

## HTTP APIs

- `GET /api/health`: handler health and start time.
- `GET /api/app`: app name, agent count, and session-service status.
- `GET /api/agents`: registered agent summaries.
- `GET /api/agents/{agent_id}`: one registered agent summary.
- `POST /api/runs`: run a registered agent with an input `model.Content`.

Minimal run request:

```json
{
  "agent_id": "deepseek_agent",
  "app_name": "embedded-example",
  "user_id": "local-user",
  "session_id": "session-1",
  "input": {
    "role": "user",
    "content": "Hi"
  }
}
```

By default the response includes a `run_id`, the active `session_id`, and the
collected ADK events. Send `Accept: text/event-stream` to receive `event`,
`partial`, `error`, and `done` SSE frames as the run progresses. Completed JSON
responses omit partial events.
