package demo

import (
	"context"
	"fmt"
	"iter"
	"os"
	"strings"

	adkagent "github.com/soasurs/adk/agent"
	"github.com/soasurs/adk/model"
	"github.com/soasurs/adk/model/deepseek"
)

func Addr() string {
	addr := strings.TrimSpace(os.Getenv("STUDIO_ADDR"))
	if addr == "" {
		return ":18080"
	}
	return addr
}

func URL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	return "http://" + addr
}

func DeepSeekFactoryFromEnv() (func() model.LLM, string, error) {
	apiKey := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	if apiKey == "" {
		return nil, "", fmt.Errorf("DEEPSEEK_API_KEY is required")
	}

	modelName := strings.TrimSpace(os.Getenv("DEEPSEEK_MODEL"))
	if modelName == "" {
		modelName = deepseek.ModelV4Flash
	}

	return func() model.LLM {
		return deepseek.New(apiKey, modelName)
	}, modelName, nil
}

type EchoAgent struct {
	NameValue        string
	DescriptionValue string
}

func NewEchoAgent(name, description string) adkagent.Agent {
	return EchoAgent{
		NameValue:        name,
		DescriptionValue: description,
	}
}

func (a EchoAgent) Name() string {
	return a.NameValue
}

func (a EchoAgent) Description() string {
	return a.DescriptionValue
}

func (a EchoAgent) Run(ctx context.Context, events []model.Event) iter.Seq2[*model.Event, error] {
	return func(yield func(*model.Event, error) bool) {
		select {
		case <-ctx.Done():
			yield(nil, ctx.Err())
			return
		default:
		}

		latest := LatestUserText(events)
		yield(&model.Event{
			Author: a.Name(),
			Content: model.Content{
				Role:    model.RoleAssistant,
				Content: "Echo: " + latest,
			},
		}, nil)
	}
}

func LatestUserText(events []model.Event) string {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Content.Role == model.RoleUser {
			return events[i].Content.Content
		}
	}
	return ""
}
