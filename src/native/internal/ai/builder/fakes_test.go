package builder

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeConversation records what the workshop writes and serves the chats it is given.
type fakeConversation struct {
	chats   map[string]chat.Chat
	sent    []string
	notes   []chat.Notice
	bound   map[string]string
	noteErr error
}

func newFakeConversation(chats ...chat.Chat) *fakeConversation {
	c := &fakeConversation{chats: map[string]chat.Chat{}, bound: map[string]string{}}
	for _, record := range chats {
		c.chats[record.ID] = record
	}
	return c
}

func (c *fakeConversation) Chat(id string) (chat.Chat, bool) {
	record, ok := c.chats[id]
	return record, ok
}

func (c *fakeConversation) Owners(extension string) []chat.Chat {
	var owners []chat.Chat
	for _, record := range c.chats {
		if record.Kind == chat.KindBuilder && record.Extension == extension {
			owners = append(owners, record)
		}
	}
	return owners
}

func (c *fakeConversation) Send(id, text string) error {
	c.sent = append(c.sent, text)
	return nil
}

func (c *fakeConversation) Note(id string, notice chat.Notice) error {
	c.notes = append(c.notes, notice)
	return c.noteErr
}

func (c *fakeConversation) Bind(id, extension string) error {
	c.bound[id] = extension
	record := c.chats[id]
	record.Extension = extension
	c.chats[id] = record
	return nil
}

// fakeExtensions scripts the checks and records the installs. checks are returned in order; the last one repeats.
type fakeExtensions struct {
	installed map[string]Candidate
	enabled   map[string]bool
	existing  map[string]api.Manifest
	source    map[string]string
	checks    []checkResult
	checked   int
}

type checkResult struct {
	methods []string
	err     error
}

func newFakeExtensions(checks ...checkResult) *fakeExtensions {
	return &fakeExtensions{
		installed: map[string]Candidate{},
		enabled:   map[string]bool{},
		existing:  map[string]api.Manifest{},
		source:    map[string]string{},
		checks:    checks,
	}
}

func (e *fakeExtensions) Definition(id string) (api.Manifest, string, error) {
	manifest, ok := e.existing[id]
	if !ok {
		return api.Manifest{}, "", errors.New("no extension with that id is installed")
	}
	return manifest, e.source[id], nil
}

func (e *fakeExtensions) Check(id string, files map[string][]byte) ([]string, error) {
	result := e.checks[len(e.checks)-1]
	if e.checked < len(e.checks) {
		result = e.checks[e.checked]
	}
	e.checked++
	return result.methods, result.err
}

func (e *fakeExtensions) Install(id string, files map[string][]byte, enable bool) error {
	e.installed[id] = Candidate{ManifestText: string(files["extension.json"]), Source: string(files["main.go"])}
	e.enabled[id] = enable
	return nil
}

// awaitUI runs the next function the workshop hands to the UI thread, failing the test if none comes.
func awaitUI(t *testing.T, queue chan func()) {
	t.Helper()
	select {
	case fn := <-queue:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("the workshop never handed its result to the UI thread")
	}
}

func newWorkshopFor(conversation *fakeConversation, extensions *fakeExtensions) (*Workshop, chan func()) {
	queue := make(chan func(), 4)
	return New(conversation, extensions, func(fn func()) { queue <- fn }), queue
}

// reply builds a finished-reply text with a manifest for id and the given source.
func reply(id, source string) string {
	return "Here it is.\n\n```json\n{\"apiVersion\":\"1.0\",\"id\":\"" + id + "\",\"title\":\"Clock\"}\n```\n\n```go\n" + source + "\n```\n"
}

const goodSource = "package main\n\nimport \"github.com/pablopunk/nvm/native/internal/ext/api\"\n\nfunc RootItems(ctx api.Context) ([]api.Item, error) { return nil, nil }\n"

func builderChat(id string) chat.Chat {
	return chat.Chat{ID: id, Kind: chat.KindBuilder, Tier: chat.TierSmart, Messages: []chat.Message{
		{Author: chat.AuthorUser, Text: "make a clock", Status: chat.StatusComplete},
		{Author: chat.AuthorAssistant, Text: reply("user.clock", goodSource), Status: chat.StatusComplete},
	}}
}

func finished() chat.Outcome {
	return chat.Outcome{Ending: chat.EndFinished}
}
