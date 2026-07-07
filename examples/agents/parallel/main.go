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
	"github.com/soasurs/adk/agent/parallelagent"
	"github.com/soasurs/adk/model"
	"github.com/soasurs/adk/session/memory"
)

func main() {
	ctx := context.Background()
	newLLM, modelName, err := demo.DeepSeekFactoryFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	agent, err := newParallelReviewAgent(newLLM)
	if err != nil {
		log.Fatal(err)
	}

	app := studio.NewApp(studio.AppConfig{Name: "parallelagent-example", LogLevel: studio.LogLevelInfo})
	app.MustRegisterAgent(agent)
	if err := app.UseSessionService(memory.NewMemorySessionService()); err != nil {
		log.Fatal(err)
	}

	addr := demo.Addr()
	log.Printf("ParallelAgent example listening on %s with DeepSeek model %s", demo.URL(addr), modelName)
	log.Printf("Try: 请评估：把所有 session 都放在内存里是否适合生产环境？")
	if err := studio.Serve(ctx, app, addr); err != nil {
		log.Fatal(err)
	}
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

const parallelRiskInstruction = `You are an independent reviewer in an ADK ParallelAgent example.

Inspect the user's request for risks, ambiguity, hidden assumptions, and missing checks.
Answer in Chinese with at most three bullets.`

const parallelSolutionInstruction = `You are an independent solution reviewer in an ADK ParallelAgent example.

Propose a direct implementation or decision path for the user's request.
Answer in Chinese with at most three bullets.`

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
