package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	studio "github.com/soasurs/adk-studio"
	"github.com/soasurs/adk-studio/examples/internal/demo"
	adkagent "github.com/soasurs/adk/agent"
	"github.com/soasurs/adk/agent/llmagent"
	"github.com/soasurs/adk/model"
	"github.com/soasurs/adk/session/memory"
	"github.com/soasurs/adk/tool"
)

func main() {
	ctx := context.Background()
	newLLM, modelName, err := demo.DeepSeekFactoryFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	tools, err := toolLabTools()
	if err != nil {
		log.Fatal(err)
	}

	app := studio.NewApp(studio.AppConfig{Name: "llmagent-example", LogLevel: studio.LogLevelInfo})
	app.MustRegisterAgent(newToolLabAgent(newLLM(), tools))
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		log.Fatal(err)
	}

	addr := demo.Addr()
	log.Printf("LLMAgent example listening on %s with DeepSeek model %s", demo.URL(addr), modelName)
	log.Printf("Registered tools: %s", strings.Join(toolNames(tools), ", "))
	log.Printf("Try: 帮我检查 Alex 的订单，看看为什么发货延迟，并给一个处理建议。")
	if err := studio.Serve(ctx, app, addr); err != nil {
		log.Fatal(err)
	}
}

func newToolLabAgent(llm model.LLM, tools []tool.Tool) adkagent.Agent {
	return llmagent.New(llmagent.Config{
		Name:        "llm_agent",
		Description: "LLMAgent example backed by DeepSeek with local fixture tools.",
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

const toolLabInstruction = `You are an ADK Studio LLMAgent example.

For any user request about a customer, order, delivery, refund, or the tool-call test scenario, use the local tools in this exact order:
1. Call lookup_customer with the user's request.
2. After lookup_customer returns, call inspect_order with the customer_id, active_order_id, and diagnostic_token from that result.
3. After inspect_order returns, call recommend_resolution with the customer_id, order_id, issue_code, and resolution_key from that result.
4. Only after all three tools have returned, answer the user concisely in Chinese.

Do not guess IDs or skip steps. Use only one tool call per assistant turn so ADK Studio can show multiple tool-call rounds.`

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

func toolNames(tools []tool.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Definition().Name)
	}
	return names
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
