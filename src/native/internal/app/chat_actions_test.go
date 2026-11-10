package app

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aichats"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestOpenedChatMenuComesFromTheBuiltInWithShortcuts runs the built-in's open action through the manager the way the
// palette does, and checks the transcript's menu the palette draws: the titles in order, and a key bound to Ask Again.
func TestOpenedChatMenuComesFromTheBuiltInWithShortcuts(t *testing.T) {
	p := goldenPalette(t)
	m := p.extensionManager()
	result, failure, err := m.Run(context.Background(), aichats.ID, api.Invocation{Ref: aichats.OpenChatRef("c-release")})
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("open chat: view=%v failure=%v err=%v", result.View != nil, failure, err)
	}
	page := extview.NewSession(p.extensionViewDeps(m), aichats.ID, *result.View).Palette()
	actions := page.Actions()
	var titles []string
	var askAgain search.Action
	for _, action := range actions {
		if strings.HasPrefix(action.Title, "Copy message ") {
			// The palette appends one row per transcript message after the built-in's menu. Users bind their own keys.
			if action.Label != "" {
				t.Errorf("%q is bound to %q; per-message rows carry no default shortcut", action.Title, action.Label)
			}
			continue
		}
		titles = append(titles, action.Title)
		if action.Title == "Ask Again" {
			askAgain = action
		}
	}
	if len(titles) == 0 || titles[0] != "Ask Again" {
		t.Fatalf("menu = %v, want Ask Again first", titles)
	}
	if !reflect.DeepEqual(titles[len(titles)-2:], []string{"New Chat", "Delete Chat"}) {
		t.Fatalf("menu ends %v, want New Chat then Delete Chat", titles)
	}
	if askAgain.Label == "" {
		t.Fatalf("Ask Again has no key label; the shortcut did not bind")
	}
}
