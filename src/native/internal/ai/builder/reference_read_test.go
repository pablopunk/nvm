package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type referenceExtensions struct {
	*fakeExtensions
	source string
}

func (e *referenceExtensions) EditSnapshot(string) (map[string][]byte, any, error) {
	return nil, nil, ErrReferenceOnly
}

func (e *referenceExtensions) Definition(id string) (api.Manifest, string, error) {
	manifest, _, err := e.fakeExtensions.Definition(id)
	return manifest, e.source, err
}

func TestRegisteredReferenceReadPreservesCapabilityWithoutWriteAuthority(t *testing.T) {
	record := builderChat("chat-1")
	record.Extension = "user.clock"
	conversation := newFakeConversation(record)
	extensions := &referenceExtensions{newFakeExtensions(), strings.Repeat("original-source\n", apiPageBytes)}
	extensions.existing["user.clock"] = clockManifestValue(t)
	w := New(conversation, extensions, synchronous)
	turn := w.Toolset(record.ID)
	conversation.active[record.ID] = turn.BuilderTurn
	read, _ := turn.Registry.Lookup(readExtensionToolName)
	first, err := read.Run(context.Background(), json.RawMessage(`{"id":"user.clock"}`))
	if err != nil || !strings.Contains(first.Text, "Reference only") || !strings.Contains(first.Text, "original-source") {
		t.Fatalf("reference: %q, %v", first.Text, err)
	}
	// Reference continuation reads the same captured definition even if its source changes.
	text, err := w.referenceText(record.Extension)
	if err != nil {
		t.Fatal(err)
	}
	offset := pageEnd(text, 0)
	extensions.source = "changed source"
	for offset < len(text) {
		next, err := read.Run(context.Background(), json.RawMessage(fmt.Sprintf(`{"id":"user.clock","offset":%d}`, offset)))
		if err != nil || !strings.Contains(next.Text, "original-source") || strings.Contains(next.Text, "changed source") {
			t.Fatalf("reference continuation: %q, %v", next.Text, err)
		}
		offset = pageEnd(text, offset)
	}
	write, _ := turn.Registry.Lookup(writeToolName)
	if _, err := write.Run(context.Background(), clockArgs(t)); err == nil {
		t.Fatal("reference pages authorized replacement")
	}
	if len(extensions.installs) != 0 {
		t.Fatal("reference-only target replaced")
	}
}
