# ADK Studio

[`github.com/soasurs/adk`](https://github.com/soasurs/adk) 的开发 Studio。

[English](./README.md)

ADK Studio 是一个可嵌入的 React 工作台，用于开发、测试和观察 ADK agent。用户在正常 Go 代码里定义 agent，把 agent 注册到 Studio，然后从同一个进程里提供 Studio UI。Studio 不负责动态加载任意 Go 源码。

## UI 预览

![ADK Studio UI](./docs/images/Screenshot.png)

## 架构

这个仓库分成三部分：

- 根目录 Go package `studio`：可嵌入的 Studio app、HTTP handler 和静态 UI 服务。
- `frontend`：React + Vite 前端。不使用 Next.js。
- `examples`：展示用户项目如何托管 Studio 的可运行示例。

运行时边界是：

```text
React Studio UI
        ↓ 同一个 HTTP server
studio.NewHandler(app)
        ↓
github.com/soasurs/adk Runner + Agent + Tools + Session
```

`frontend/dist` 是生成出来的构建产物，已经被 Git 忽略。编译会嵌入 UI 的 Go 代码之前，需要先构建前端。

## 当前范围

这还是一个偏骨架阶段的 Studio，但核心链路已经可以工作：

- 在 `studio.App` 中注册一个或多个 ADK agent。
- 配置 session service，用于多轮运行。
- 通过 `/api/agents` 发现已注册 agent。
- 通过 `POST /api/runs` 执行选中的 agent。
- 在 trace 面板里查看 ADK events 和运行时 span records。
- 在 UI 中分别展示 assistant message、reasoning content、tool call 和 tool result。
- Studio UI 执行 agent 时，可以选择是否通过 SSE 实时接收 ADK events。
- 使用固定高度的 React 工作台，包含侧边栏控制区、playground、trace inspector，以及可切换的发送快捷键。

run API 会保留普通客户端使用的完整 JSON 响应。发送
`Accept: text/event-stream` 的客户端会在 ADK event 和运行时 span record
产出时收到实时 SSE frame。两种传输方式都会保留同一条运行时 feed 的顺序。

## 构建

先安装并构建前端：

```bash
cd frontend
pnpm install
pnpm build
cd ..
```

然后再构建或测试 Go package：

```bash
go test ./...
go build ./...
```

Go package 使用 `go:embed` 嵌入 `frontend/dist`。所以在全新 checkout 后，需要先构建前端，`go test`、`go build` 或 `go run` 才能编译 handler。

## 运行示例

每个示例只聚焦一个 ADK Studio 维度。所有示例默认监听 `:18080`，可以通过
`STUDIO_ADDR` 改地址。

agent 类型示例使用 DeepSeek，需要先配置：

```bash
export DEEPSEEK_API_KEY=...
# 可选：
export DEEPSEEK_MODEL=...
```

一次运行一种 agent 类型：

```bash
go run ./examples/agents/llm
go run ./examples/agents/sequential
go run ./examples/agents/parallel
```

- `examples/agents/llm`：注册一个基于 DeepSeek 的 `llmagent`，带本地 fixture tools。
  可以试：`帮我检查 Alex 的订单，看看为什么发货延迟，并给一个处理建议。`
- `examples/agents/sequential`：注册一个 `sequentialagent`，先运行带 `read_file`
  的 researcher 子 agent，再运行 writer 子 agent。可以试：
  `请用 read_file 读取 README.md 和 examples/agents/sequential/main.go，然后总结这个示例的流程。`
- `examples/agents/parallel`：注册一个 `parallelagent`，并发运行两个 reviewer
  并合并答案。可以试：`请评估：把所有 session 都放在内存里是否适合生产环境？`

session backend 示例使用确定性的 echo agent，这样唯一变化就是 session store：

```bash
go run ./examples/sessions/memory
go run ./examples/sessions/sqlite
ADK_STUDIO_POSTGRES_DSN=postgres://... go run ./examples/sessions/postgres
```

- `examples/sessions/memory`：使用 `session/memory`。
- `examples/sessions/sqlite`：使用 ADK `session/database` 和 SQLite。可通过
  `ADK_STUDIO_SQLITE_DSN` 覆盖默认本地数据库文件。
- `examples/sessions/postgres`：使用 ADK `session/database` 和 PostgreSQL。
  多进程部署还应该提供分布式 run locker。

启动任意示例后，打开 [http://127.0.0.1:18080](http://127.0.0.1:18080)。

## 前端开发

先让任意 Go 示例运行在 `:18080`，再启动 Vite：

```bash
cd frontend
pnpm dev
```

Vite dev server 会把 `/api/*` 请求代理到 `http://127.0.0.1:18080`。

生产构建使用：

```bash
cd frontend
pnpm build
```

`frontend/dist` 下的生成文件不应该提交。

## 嵌入 Studio

用户项目中的集成方式大致如下：

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

如果需要自己控制 HTTP server，也可以直接挂 handler：

```go
http.ListenAndServe(":18080", studio.NewHandler(app))
```

Studio 默认使用 Go 标准库 `log/slog` 的 text handler，把日志以 INFO 级别写到
`stderr`。每次 run 返回的 ADK event 都会按 INFO 打印。嵌入时可以通过
`LogLevelDebug`、`LogLevelWarn`、`LogLevelError`、`LogLevelOff` 调整等级，也可以在
`AppConfig.Logger` 里传入自定义 `*slog.Logger`。

Studio 始终会为 Trace 面板收集 ADK 运行时 spans。如果还需要把同一批 spans
转发给宿主的可观测性系统，可以配置 ADK `trace.Tracer`；Studio 会保留这个
tracer 返回的 context，继续传给下游 spans：

```go
app := studio.NewApp(studio.AppConfig{
    Name:   "demo",
    Tracer: hostTracer,
})
```

`Tracer` 为 nil 只会关闭宿主侧转发，不会额外产生 span 日志，也不会关闭
Studio UI collector。

## HTTP APIs

- `GET /api/health`：handler 健康状态和启动时间。
- `GET /api/app`：app 名称、agent 数量、session service 状态和当前 session
  backend summary。
- `GET /api/agents`：已注册 agent 列表。
- `GET /api/agents/{agent_id}`：单个已注册 agent 信息。
- `POST /api/runs`：用一个 `model.Content` 输入运行已注册 agent。

最小 run request：

```json
{
  "agent_id": "llm_agent",
  "app_name": "llmagent-example",
  "user_id": "local-user",
  "session_id": "session-1",
  "input": {
    "role": "user",
    "content": "Hi"
  }
}
```

默认响应里会包含 Studio `run_id`、当前 `session_id` 和有序的 run feed。
发送 `Accept: text/event-stream` 可以在运行过程中实时收到 `trace`、`event`、
`partial`、`error` 和 `done` SSE frames。完整 JSON 响应同样包含运行时 trace
records 和终止 frame，但会省略 partial model events。

`trace` frame 使用稳定的 snake_case 结构，包含 span `phase`（`start`、
`event` 或 `end`）、`kind`、时间、duration、ADK runtime run/turn/session 标识，
以及相关的 agent、model、tool、event、token 和 error 字段。tool span 会保留
`tool_index: 0`。ADK `model.Event` payload 也会同步暴露 `TurnID` 和完整 token
usage details。

终止响应继续保留顶层 `error` 字符串和 HTTP 500 行为。最后一个 `error` frame
还会携带类型化 `failure`，例如 `run_failed` 或 `tool_execution_unknown`。
unknown tool execution 的 details 会包含来源 turn/event 和未决 call 的 ID/name，
但不会暴露 tool arguments。runtime event ID 和 source event ID 使用十进制字符串，
避免 JavaScript 客户端丢失 64 位 Snowflake ID 的精度。

ADK v0.0.10 会回滚 incomplete turn 中写入的所有 events。SSE 客户端断开或写入
失败时，Studio 会取消运行并等待 Runner 退出，确保 handler 返回前完成回滚。
在 UI 里，失败轮次会保留用户本地输入并标记为 `Failed · not persisted`，移除
属于该 Studio run 的 assistant/tool 消息，同时在 Trace 中保留完整尝试、终止
错误和 spans。Studio 不会自动重试 `tool_execution_unknown` calls；应新建
session，或先修复持久化历史再继续使用原 session。
