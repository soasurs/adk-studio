package main

import (
	"context"
	"log"
	"os"

	studio "github.com/soasurs/adk-studio"
	"github.com/soasurs/adk-studio/examples/internal/demo"
	adkagent "github.com/soasurs/adk/agent"
	"github.com/soasurs/adk/agent/llmagent"
	"github.com/soasurs/adk/agent/sequentialagent"
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
	workspaceRoot, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := newSequentialPipelineAgent(newLLM, workspaceRoot)
	if err != nil {
		log.Fatal(err)
	}

	app := studio.NewApp(studio.AppConfig{Name: "sequentialagent-example", LogLevel: studio.LogLevelInfo})
	app.MustRegisterAgent(agent)
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		log.Fatal(err)
	}

	addr := demo.Addr()
	log.Printf("SequentialAgent example listening on %s with DeepSeek model %s", demo.URL(addr), modelName)
	log.Printf("read_file root: %s", workspaceRoot)
	log.Printf("Try: 请用 read_file 读取 README.md 和 examples/agents/sequential/main.go，然后总结这个示例的流程。")
	if err := studio.Serve(ctx, app, addr); err != nil {
		log.Fatal(err)
	}
}

func newSequentialPipelineAgent(newLLM func() model.LLM, workspaceRoot string) (adkagent.Agent, error) {
	readFile, err := demo.NewReadFileTool(workspaceRoot)
	if err != nil {
		return nil, err
	}

	researcher := newResearcherAgent(newLLM(), readFile)
	writer := newWriterAgent(newLLM())
	return sequentialagent.New(sequentialagent.Config{
		Name:        "sequential_pipeline_agent",
		Description: "SequentialAgent example that runs a researcher and writer in order.",
		Agents:      []adkagent.Agent{researcher, writer},
	})
}

func newResearcherAgent(llm model.LLM, readFile tool.Tool) adkagent.Agent {
	return llmagent.New(llmagent.Config{
		Name:        "pipeline_researcher",
		Description: "First step in the sequential pipeline; reads files and prepares a handoff note.",
		Model:       llm,
		Tools:       []tool.Tool{readFile},
		Instruction: sequentialResearchInstruction,
		GenerateConfig: &model.GenerateConfig{
			Temperature: 0.2,
			MaxTokens:   1024,
		},
		MaxIterations: 30,
		Stream:        true,
	})
}

func newWriterAgent(llm model.LLM) adkagent.Agent {
	return llmagent.New(llmagent.Config{
		Name:        "pipeline_writer",
		Description: "Second step in the sequential pipeline; writes the final answer.",
		Model:       llm,
		Instruction: sequentialWriterInstruction,
		GenerateConfig: &model.GenerateConfig{
			Temperature: 0.3,
			MaxTokens:   768,
		},
		Stream: true,
	})
}

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
