package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	var expected []string
	for _, name := range api.KnownCapabilities() {
		expected = append(expected, string(name))
	}
	sort.Strings(expected)
	body := collectCapabilityPages(t, func(offset int) (string, error) {
		args, _ := json.Marshal(readArgs{Offset: offset})
		page, err := tool.Run(context.Background(), args)
		return page.Text, err
	})
	if body != strings.Join(expected, "\n") {
		t.Fatal("discovery did not reconstruct the sorted registry")
	}
	if !strings.Contains(tool.Description, "Runtime probe names can differ") {
		t.Fatal("description conflates manifest declarations and runtime probes")
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

func TestCapabilityPagesContinueAndReconstructLargeCatalog(t *testing.T) {
	body := strings.Repeat("known.declaration\n", 2*apiPageBytes/10)
	calls := 0
	actual := collectCapabilityPages(t, func(offset int) (string, error) {
		calls++
		result, err := capabilityPage(body, offset)
		return result.Text, err
	})
	if calls < 2 || actual != body {
		t.Fatal("continuation lost or repeated declaration bytes")
	}
}

func collectCapabilityPages(t *testing.T, read func(int) (string, error)) string {
	t.Helper()
	var body strings.Builder
	for offset := 0; ; {
		text, err := read(offset)
		if err != nil {
			t.Fatal(err)
		}
		if len(text) > apiPageBytes+512 {
			t.Fatal("page exceeds output bound")
		}
		parts := strings.SplitN(text, "\n\n", 2)
		if len(parts) != 2 {
			t.Fatal("page has no content boundary")
		}
		body.WriteString(parts[1])
		next := offset + len(parts[1])
		if !strings.Contains(parts[0], "Continue with offset ") {
			break
		}
		if next <= offset || !strings.Contains(parts[0], fmt.Sprintf("Continue with offset %d.", next)) {
			t.Fatal("continuation does not advance to the first unread byte")
		}
		offset = next
	}
	return body.String()
}
