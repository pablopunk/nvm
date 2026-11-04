package child

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestUnavailableBuilderActionsValidate keeps a view that carries a builder action loadable: the host rejects an
// action whose kind it does not know, so each placeholder must validate.
func TestUnavailableBuilderActionsValidate(t *testing.T) {
	builder := unsupportedAIBuilder{}
	actions := map[string]api.Action{
		"startChat":      builder.StartChat("prompt", "Start chat"),
		"openChat":       builder.OpenChat("chat"),
		"removeChat":     builder.RemoveChat("chat"),
		"tweakExtension": builder.TweakExtension(api.TweakRequest{}),
		"openChatsList":  builder.OpenChatsList("Chats"),
	}
	for name, action := range actions {
		if action.Kind != api.ActionUnavailable {
			t.Fatalf("%s kind %q; want %q", name, action.Kind, api.ActionUnavailable)
		}
		if err := action.Validate(); err != nil {
			t.Fatalf("%s does not validate: %v", name, err)
		}
	}
}
