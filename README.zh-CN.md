# ADK Studio

[`github.com/soasurs/adk`](https://github.com/soasurs/adk) 的开发 Studio。

[English](./README.md)

ADK Studio 是一个可嵌入的 React 工作台，用于开发、测试和观察 ADK agent。用户在正常 Go 代码里定义 agent，把 agent 注册到 Studio，然后从同一个进程里提供 Studio UI。Studio 不负责动态加载任意 Go 源码。

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
- 在 trace 面板里查看返回的 ADK events。
- 在 UI 中分别展示 assistant message、reasoning content、tool call 和 tool result。
- Studio UI 执行 agent 时，可以选择是否通过 SSE 实时接收 ADK events。
- 使用固定高度的 React 工作台，包含侧边栏控制区、playground、trace inspector，以及可切换的发送快捷键。

run API 会保留普通客户端使用的完整 JSON 响应。发送
`Accept: text/event-stream` 的客户端会在每个 ADK event 产出时收到实时
SSE frame。

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

embedded 示例注册了三种 ADK agent 类型：

- `deepseek_agent`：基于 DeepSeek 的 `llmagent`，带有本地 fixture tools 和
  Exa MCP 搜索 tools。
- `sequential_pipeline_agent`：一个 `sequentialagent`，会依次运行 researcher
  子 agent 和 writer 子 agent；researcher 带有真实的 `read_file` tool。
- `parallel_review_agent`：一个 `parallelagent`，会并发运行两个 reviewer 子
  agent，然后合并最终答案。

```bash
export DEEPSEEK_API_KEY=...
# 可选：
export DEEPSEEK_MODEL=...
export EXA_API_KEY=...

go run ./examples/embedded
```

打开 [http://127.0.0.1:18080](http://127.0.0.1:18080)。

可以试这些 prompt：

```text
帮我检查 Alex 的订单，看看为什么发货延迟，并给一个处理建议。
用 Exa 搜索 github.com/soasurs/adk 的相关信息，并总结来源。
请用 read_file 读取 README.md 和 examples/embedded/main.go，然后分析这个示例展示了哪些 agent 类型。
请评估：把所有 session 都放在内存里是否适合生产环境？
```

前两个 prompt 适合用 `deepseek_agent`。本地 tool prompt 用来测试多轮 tool call：
`lookup_customer` → `inspect_order` → `recommend_resolution`。

第三个 prompt 适合用 `sequential_pipeline_agent`，可以看到 researcher 子 agent 先调用
`read_file`，再把实际读取结果交给 writer。`read_file` 被限制在示例进程启动时的当前工作目录内。
第四个 prompt 适合用 `parallel_review_agent`，可以看到并发 fan-out 后的合并结果。

## 前端开发

先让 Go 示例运行在 `:18080`，再启动 Vite：

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

## HTTP APIs

- `GET /api/health`：handler 健康状态和启动时间。
- `GET /api/app`：app 名称、agent 数量和 session service 状态。
- `GET /api/agents`：已注册 agent 列表。
- `GET /api/agents/{agent_id}`：单个已注册 agent 信息。
- `POST /api/runs`：用一个 `model.Content` 输入运行已注册 agent。

最小 run request：

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

默认响应里会包含 `run_id`、当前 `session_id` 和收集到的 ADK events。
发送 `Accept: text/event-stream` 可以在运行过程中实时收到 `event`、`partial`、
`error` 和 `done` SSE frames。完整 JSON 响应会省略 partial events。
