package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	studio "github.com/soasurs/adk-studio"
	adkagent "github.com/soasurs/adk/agent"
	"github.com/soasurs/adk/agent/llmagent"
	"github.com/soasurs/adk/agent/parallelagent"
	"github.com/soasurs/adk/agent/sequentialagent"
	"github.com/soasurs/adk/model"
	"github.com/soasurs/adk/model/deepseek"
	"github.com/soasurs/adk/session/memory"
	"github.com/soasurs/adk/tool"
	adkmcp "github.com/soasurs/adk/tool/mcp"
)

const exaMCPEndpoint = "https://mcp.exa.ai/mcp"

const (
	defaultReadFileMaxBytes = 16 * 1024
	hardReadFileMaxBytes    = 64 * 1024
)

func main() {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY is required")
	}

	modelName := os.Getenv("DEEPSEEK_MODEL")
	if modelName == "" {
		modelName = deepseek.ModelV4Flash
	}

	ctx := context.Background()
	workspaceRoot, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	newLLM := func() model.LLM {
		return deepseek.New(apiKey, modelName)
	}
	localTools, err := toolLabTools()
	if err != nil {
		log.Fatal(err)
	}
	exaToolSet, exaTools, err := exaSearchTools(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer exaToolSet.Close()

	tools := append(localTools, exaTools...)
	agents, err := exampleAgents(newLLM, tools, workspaceRoot)
	if err != nil {
		log.Fatal(err)
	}

	app := studio.NewApp(studio.AppConfig{Name: "embedded-example", LogLevel: studio.LogLevelInfo})
	for _, a := range agents {
		app.MustRegisterAgent(a)
	}
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		log.Fatal(err)
	}

	log.Printf("ADK Studio example listening on http://127.0.0.1:18080 with DeepSeek model %s", modelName)
	log.Printf("Registered agents: %s", strings.Join(agentNames(agents), ", "))
	log.Printf("Registered deepseek_agent tools: %s", strings.Join(toolNames(tools), ", "))
	log.Printf("sequential_pipeline_agent read_file root: %s", workspaceRoot)
	log.Printf("Try deepseek_agent local tools: 帮我检查 Alex 的订单，看看为什么发货延迟，并给一个处理建议。")
	log.Printf("Try deepseek_agent Exa MCP: 用 Exa 搜索 github.com/soasurs/adk 的相关信息，并总结来源。")
	log.Printf("Try sequential_pipeline_agent: 请用 read_file 读取 README.md 和 examples/embedded/main.go，然后分析这个示例展示了哪些 agent 类型。")
	log.Printf("Try parallel_review_agent: 请评估：把所有 session 都放在内存里是否适合生产环境？")
	if err := studio.Serve(ctx, app, ":18080"); err != nil {
		log.Fatal(err)
	}
}

func exampleAgents(newLLM func() model.LLM, tools []tool.Tool, workspaceRoot string) ([]adkagent.Agent, error) {
	sequential, err := newSequentialPipelineAgent(newLLM, workspaceRoot)
	if err != nil {
		return nil, err
	}
	parallel, err := newParallelReviewAgent(newLLM)
	if err != nil {
		return nil, err
	}

	return []adkagent.Agent{
		newToolLabAgent(newLLM(), tools),
		sequential,
		parallel,
	}, nil
}

func newToolLabAgent(llm model.LLM, tools []tool.Tool) adkagent.Agent {
	return llmagent.New(llmagent.Config{
		Name:        "deepseek_agent",
		Description: "DeepSeek-backed LLMAgent with local fixtures and Exa MCP search.",
		Model:       llm,
		Tools:       tools,
		Instruction: toolLabInstruction,
		GenerateConfig: &model.GenerateConfig{
			Temperature: 0.2,
			MaxTokens:   1024,
		},
		MaxIterations: 8,
		Stream:        true,
	})
}

func newSequentialPipelineAgent(newLLM func() model.LLM, workspaceRoot string) (adkagent.Agent, error) {
	readFile, err := newReadFileTool(workspaceRoot)
	if err != nil {
		return nil, err
	}

	researcher := llmagent.New(llmagent.Config{
		Name:        "pipeline_researcher",
		Description: "First step in the sequential pipeline; reads files and prepares a handoff note.",
		Model:       newLLM(),
		Tools:       []tool.Tool{readFile},
		Instruction: sequentialResearchInstruction,
		GenerateConfig: &model.GenerateConfig{
			Temperature: 0.2,
			MaxTokens:   1024,
		},
		MaxIterations: 6,
		Stream:        true,
	})
	writer := llmagent.New(llmagent.Config{
		Name:        "pipeline_writer",
		Description: "Second step in the sequential pipeline; writes the final answer.",
		Model:       newLLM(),
		Instruction: sequentialWriterInstruction,
		GenerateConfig: &model.GenerateConfig{
			Temperature: 0.3,
			MaxTokens:   768,
		},
		Stream: true,
	})

	return sequentialagent.New(sequentialagent.Config{
		Name:        "sequential_pipeline_agent",
		Description: "SequentialAgent example that runs a researcher and writer in order.",
		Agents:      []adkagent.Agent{researcher, writer},
	})
}

func newParallelReviewAgent(newLLM func() model.LLM) (adkagent.Agent, error) {
	riskReviewer := llmagent.New(llmagent.Config{
		Name:        "risk_reviewer",
		Description: "Parallel reviewer focused on risks and missing checks.",
		Model:       newLLM(),
		Instruction: parallelRiskInstruction,
		GenerateConfig: &model.GenerateConfig{
			Temperature: 0.2,
			MaxTokens:   512,
		},
	})
	solutionReviewer := llmagent.New(llmagent.Config{
		Name:        "solution_reviewer",
		Description: "Parallel reviewer focused on a direct implementation path.",
		Model:       newLLM(),
		Instruction: parallelSolutionInstruction,
		GenerateConfig: &model.GenerateConfig{
			Temperature: 0.3,
			MaxTokens:   512,
		},
	})

	return parallelagent.New(parallelagent.Config{
		Name:        "parallel_review_agent",
		Description: "ParallelAgent example that fans out to two reviewers and merges their answers.",
		Agents:      []adkagent.Agent{riskReviewer, solutionReviewer},
		MergeFunc:   mergeParallelReviewOutputs,
	})
}

const toolLabInstruction = `You are an ADK Studio tool-call test agent.

For any user request about a customer, order, delivery, refund, or the tool-call test scenario, use the local tools in this exact order:
1. Call lookup_customer with the user's request.
2. After lookup_customer returns, call inspect_order with the customer_id, active_order_id, and diagnostic_token from that result.
3. After inspect_order returns, call recommend_resolution with the customer_id, order_id, issue_code, and resolution_key from that result.
4. Only after all three tools have returned, answer the user concisely in Chinese.

For any request that asks to search, research, verify current information, or use Exa, call the available Exa MCP search tool before answering. Prefer citing URLs returned by the tool.

Do not guess IDs or skip steps. Use only one tool call per assistant turn so ADK Studio can show multiple tool-call rounds.`

const sequentialResearchInstruction = `You are the research step in an ADK SequentialAgent example.

You have a real read_file tool that reads files from the example process working directory.
When the user asks you to read, inspect, review, or analyze files, call read_file before writing your handoff.
Read one file per tool call. Do not claim that you read a file unless read_file returned its content.

Analyze the user's request and produce a compact handoff note in Chinese with:
- the likely goal,
- files actually read, when any,
- key facts or assumptions,
- risks or missing information,
- a recommended direction.

Do not write the final answer.`

const sequentialWriterInstruction = `You are the final response step in an ADK SequentialAgent example.

You receive the original user request, the previous assistant handoff note, and a "Please proceed." user message.
Use the handoff as context, then answer the original user request in Chinese.
Keep the answer concise, concrete, and action-oriented.`

const parallelRiskInstruction = `You are an independent reviewer in an ADK ParallelAgent example.

Inspect the user's request for risks, ambiguity, hidden assumptions, and missing checks.
Answer in Chinese with at most three bullets.`

const parallelSolutionInstruction = `You are an independent solution reviewer in an ADK ParallelAgent example.

Propose a direct implementation or decision path for the user's request.
Answer in Chinese with at most three bullets.`

type apiKeyTransport struct {
	base   http.RoundTripper
	header string
	value  string
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set(t.header, t.value)
	return t.base.RoundTrip(clone)
}

type lookupCustomerInput struct {
	Query string `json:"query" jsonschema:"The user's original request or the customer lookup phrase."`
}

type lookupCustomerOutput struct {
	CustomerID      string   `json:"customer_id"`
	CustomerName    string   `json:"customer_name"`
	ActiveOrderID   string   `json:"active_order_id"`
	DiagnosticToken string   `json:"diagnostic_token"`
	Notes           []string `json:"notes"`
}

type inspectOrderInput struct {
	CustomerID      string `json:"customer_id" jsonschema:"The customer_id returned by lookup_customer."`
	OrderID         string `json:"order_id" jsonschema:"The active_order_id returned by lookup_customer."`
	DiagnosticToken string `json:"diagnostic_token" jsonschema:"The diagnostic_token returned by lookup_customer."`
}

type inspectOrderOutput struct {
	OrderID       string   `json:"order_id"`
	Status        string   `json:"status"`
	IssueCode     string   `json:"issue_code"`
	ResolutionKey string   `json:"resolution_key"`
	Items         []string `json:"items"`
	Observations  []string `json:"observations"`
}

type recommendResolutionInput struct {
	CustomerID    string `json:"customer_id" jsonschema:"The customer_id returned by lookup_customer."`
	OrderID       string `json:"order_id" jsonschema:"The order_id returned by inspect_order."`
	IssueCode     string `json:"issue_code" jsonschema:"The issue_code returned by inspect_order."`
	ResolutionKey string `json:"resolution_key" jsonschema:"The resolution_key returned by inspect_order."`
}

type recommendResolutionOutput struct {
	Recommendation string   `json:"recommendation"`
	NextSteps      []string `json:"next_steps"`
	Confidence     string   `json:"confidence"`
}

type readFileInput struct {
	Path     string `json:"path" jsonschema:"Workspace-relative file path to read. Absolute paths and paths outside the workspace are rejected."`
	MaxBytes int64  `json:"max_bytes,omitempty" jsonschema:"Optional maximum number of bytes to return. Defaults to 16384 and is capped at 65536."`
}

type readFileOutput struct {
	Path          string `json:"path"`
	TotalBytes    int64  `json:"total_bytes"`
	ReturnedBytes int64  `json:"returned_bytes"`
	Truncated     bool   `json:"truncated"`
	Content       string `json:"content"`
}

func newReadFileTool(workspaceRoot string) (tool.Tool, error) {
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve read_file root: %w", err)
	}

	return tool.NewFunc(tool.Definition{
		Name:        "read_file",
		Description: "Read a UTF-8 text file from the example process working directory. Absolute paths and paths outside that root are rejected.",
	}, func(ctx context.Context, input readFileInput) (readFileOutput, error) {
		return readFile(ctx, absRoot, input)
	})
}

func readFile(ctx context.Context, workspaceRoot string, input readFileInput) (readFileOutput, error) {
	select {
	case <-ctx.Done():
		return readFileOutput{}, ctx.Err()
	default:
	}

	requestedPath := strings.TrimSpace(input.Path)
	if requestedPath == "" {
		return readFileOutput{}, fmt.Errorf("path is required")
	}
	if filepath.IsAbs(requestedPath) {
		return readFileOutput{}, fmt.Errorf("absolute paths are not allowed")
	}

	cleanPath := filepath.Clean(requestedPath)
	if cleanPath == "." {
		return readFileOutput{}, fmt.Errorf("path must point to a file")
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return readFileOutput{}, fmt.Errorf("paths outside the read_file root are not allowed")
	}

	maxBytes := input.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultReadFileMaxBytes
	}
	if maxBytes > hardReadFileMaxBytes {
		maxBytes = hardReadFileMaxBytes
	}

	root, err := os.OpenRoot(workspaceRoot)
	if err != nil {
		return readFileOutput{}, fmt.Errorf("open read_file root: %w", err)
	}
	defer root.Close()

	info, err := root.Stat(cleanPath)
	if err != nil {
		return readFileOutput{}, fmt.Errorf("stat %q: %w", cleanPath, err)
	}
	if info.IsDir() {
		return readFileOutput{}, fmt.Errorf("%q is a directory", cleanPath)
	}

	file, err := root.Open(cleanPath)
	if err != nil {
		return readFileOutput{}, fmt.Errorf("open %q: %w", cleanPath, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return readFileOutput{}, fmt.Errorf("read %q: %w", cleanPath, err)
	}

	truncated := int64(len(data)) > maxBytes
	if truncated {
		data = data[:maxBytes]
	}

	return readFileOutput{
		Path:          filepath.ToSlash(cleanPath),
		TotalBytes:    info.Size(),
		ReturnedBytes: int64(len(data)),
		Truncated:     truncated,
		Content:       string(data),
	}, nil
}

func toolLabTools() ([]tool.Tool, error) {
	lookup, err := tool.NewFunc(tool.Definition{
		Name:        "lookup_customer",
		Description: "Step 1 only. Resolve a customer/order test scenario into stable IDs and a diagnostic token.",
	}, lookupCustomer)
	if err != nil {
		return nil, err
	}

	inspect, err := tool.NewFunc(tool.Definition{
		Name:        "inspect_order",
		Description: "Step 2 only. Inspect the order using IDs and diagnostic_token returned by lookup_customer.",
	}, inspectOrder)
	if err != nil {
		return nil, err
	}

	recommend, err := tool.NewFunc(tool.Definition{
		Name:        "recommend_resolution",
		Description: "Step 3 only. Produce an operational recommendation from inspect_order issue details.",
	}, recommendResolution)
	if err != nil {
		return nil, err
	}

	return []tool.Tool{lookup, inspect, recommend}, nil
}

func exaSearchTools(ctx context.Context) (*adkmcp.ToolSet, []tool.Tool, error) {
	transport := &sdkmcp.StreamableClientTransport{
		Endpoint: exaMCPEndpoint,
	}
	if exaKey := os.Getenv("EXA_API_KEY"); exaKey != "" {
		transport.HTTPClient = &http.Client{
			Transport: &apiKeyTransport{
				base:   http.DefaultTransport,
				header: "x-api-key",
				value:  exaKey,
			},
		}
	} else {
		log.Printf("EXA_API_KEY is not set; connecting to Exa MCP without an API key")
	}

	toolSet := adkmcp.NewToolSet(transport)
	log.Printf("Connecting to Exa MCP at %s", exaMCPEndpoint)
	if err := toolSet.Connect(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect Exa MCP: %w", err)
	}
	tools, err := toolSet.Tools(ctx)
	if err != nil {
		_ = toolSet.Close()
		return nil, nil, fmt.Errorf("list Exa MCP tools: %w", err)
	}
	if len(tools) == 0 {
		_ = toolSet.Close()
		return nil, nil, fmt.Errorf("Exa MCP returned no tools")
	}
	log.Printf("Exa MCP tools: %s", strings.Join(toolNames(tools), ", "))
	return toolSet, tools, nil
}

func toolNames(tools []tool.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Definition().Name)
	}
	return names
}

func agentNames(agents []adkagent.Agent) []string {
	names := make([]string, 0, len(agents))
	for _, a := range agents {
		names = append(names, a.Name())
	}
	return names
}

func mergeParallelReviewOutputs(results []parallelagent.AgentOutput) model.Event {
	parts := make([]string, 0, len(results))
	for _, result := range results {
		if content := lastAssistantText(result.Events); content != "" {
			parts = append(parts, fmt.Sprintf("[%s]\n%s", result.Name, content))
		}
	}

	return model.Event{
		Author: "parallel_review_agent",
		Content: model.Content{
			Role:    model.RoleAssistant,
			Content: strings.Join(parts, "\n\n"),
		},
	}
}

func lastAssistantText(events []model.Event) string {
	for i := len(events) - 1; i >= 0; i-- {
		content := events[i].Content
		if content.Role == model.RoleAssistant && content.Content != "" {
			return content.Content
		}
	}
	return ""
}

func lookupCustomer(_ context.Context, input lookupCustomerInput) (lookupCustomerOutput, error) {
	query := strings.ToLower(input.Query)
	if strings.Contains(query, "sam") || strings.Contains(query, "bob") {
		return lookupCustomerOutput{
			CustomerID:      "cus-2048",
			CustomerName:    "Sam Rivera",
			ActiveOrderID:   "ord-7352",
			DiagnosticToken: "diag-cus-2048-ord-7352",
			Notes: []string{
				"matched fixture customer by test alias",
				"customer has one active replacement order",
			},
		}, nil
	}

	return lookupCustomerOutput{
		CustomerID:      "cus-1001",
		CustomerName:    "Alex Chen",
		ActiveOrderID:   "ord-9001",
		DiagnosticToken: "diag-cus-1001-ord-9001",
		Notes: []string{
			"matched default Studio tool-call fixture",
			"active order is flagged for delivery investigation",
		},
	}, nil
}

func inspectOrder(_ context.Context, input inspectOrderInput) (inspectOrderOutput, error) {
	if input.CustomerID == "" || input.OrderID == "" || input.DiagnosticToken == "" {
		return inspectOrderOutput{}, fmt.Errorf("customer_id, order_id, and diagnostic_token are required")
	}

	switch input.OrderID {
	case "ord-7352":
		return inspectOrderOutput{
			OrderID:       input.OrderID,
			Status:        "replacement_ready",
			IssueCode:     "replacement_pending_pickup",
			ResolutionKey: "ship-replacement-now",
			Items:         []string{"USB-C hub", "65W charger"},
			Observations: []string{
				"replacement order passed warehouse QA",
				"carrier pickup window opens this afternoon",
			},
		}, nil
	case "ord-9001":
		return inspectOrderOutput{
			OrderID:       input.OrderID,
			Status:        "delayed_in_transit",
			IssueCode:     "carrier_sort_exception",
			ResolutionKey: "expedite-or-credit",
			Items:         []string{"ADK Studio hoodie", "debug notebook"},
			Observations: []string{
				"carrier reported a sort exception at the regional hub",
				"warehouse inventory can support a replacement shipment",
				"customer is still inside the service recovery window",
			},
		}, nil
	default:
		return inspectOrderOutput{}, fmt.Errorf("unknown fixture order_id %q", input.OrderID)
	}
}

func recommendResolution(_ context.Context, input recommendResolutionInput) (recommendResolutionOutput, error) {
	if input.IssueCode == "" || input.ResolutionKey == "" {
		return recommendResolutionOutput{}, fmt.Errorf("issue_code and resolution_key are required")
	}

	switch input.ResolutionKey {
	case "ship-replacement-now":
		return recommendResolutionOutput{
			Recommendation: "安排今天的补发揽收，并把原订单继续保留观察。",
			NextSteps: []string{
				"通知仓库释放 replacement order",
				"给客户发送新的 tracking number",
				"24 小时后复查物流状态",
			},
			Confidence: "high",
		}, nil
	case "expedite-or-credit":
		return recommendResolutionOutput{
			Recommendation: "优先发起加急补发；如果客户不想等待，提供运费抵扣。",
			NextSteps: []string{
				"联系承运商确认 regional hub 的异常是否可恢复",
				"同步创建 replacement shipment",
				"向客户说明延迟原因和两个可选处理方案",
			},
			Confidence: "medium-high",
		}, nil
	default:
		return recommendResolutionOutput{}, fmt.Errorf("unknown resolution_key %q", input.ResolutionKey)
	}
}
