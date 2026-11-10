package builder

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestListCapabilitiesContract(t *testing.T) {
	w := New(newFakeConversation(), newFakeExtensions(), synchronous)
	tool, ok := w.Toolset("chat").Registry.Lookup("list_capabilities")
	if !ok {
		t.Fatal("list_capabilities is not registered")
	}
	result, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "do not indicate host support or permissions") {
		t.Fatal(result.Text)
	}
	for _, name := range api.KnownCapabilities() {
		if !strings.Contains("\n"+result.Text+"\n", "\n"+string(name)+"\n") {
			t.Errorf("missing %s", name)
		}
	}
	if len(result.Text) > apiPageBytes+512 {
		t.Fatal("result exceeds page bound")
	}
	for _, args := range []string{`{"offset":-1}`, `{"offset":999999}`, `{"offset":"bad"}`} {
		if _, err := tool.Run(context.Background(), json.RawMessage(args)); err == nil {
			t.Errorf("accepted %s", args)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tool.Run(ctx, json.RawMessage(`{}`)); err == nil {
		t.Fatal("accepted cancelled call")
	}
}
