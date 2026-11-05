package aicommands

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type fastCallAI struct {
	api.AI
	prompt string
	system string
}

func (a *fastCallAI) Fast(prompt, system string) (string, error) {
	a.prompt, a.system = prompt, system
	return "fixed", nil
}

type fastCallContext struct {
	api.Context
	ai api.AI
}

func (c fastCallContext) AI() api.AI { return c.ai }

func TestFixAskModelUsesFastWithSeparateSystemAndUserText(t *testing.T) {
	ai := &fastCallAI{}
	got, err := askModel(fastCallContext{ai: ai})("system instructions", "user text")
	if err != nil || got != "fixed" {
		t.Fatalf("askModel = %q, %v", got, err)
	}
	if ai.prompt != "user text" || ai.system != "system instructions" {
		t.Fatalf("Fast arguments = prompt %q, system %q", ai.prompt, ai.system)
	}
}
