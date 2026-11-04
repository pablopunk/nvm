package builder

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeConversation serves the chats the workshop reads, records the bindings and the steps it shows. An empty label in
// steps is a step that ended.
type fakeConversation struct {
	chats map[string]chat.Chat
	bound map[string]string
	steps []string
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

func (c *fakeConversation) Bind(id, extension string) error {
	record, ok := c.chats[id]
	if !ok {
		return chat.ErrNotFound
	}
	record.Extension = extension
	c.chats[id] = record
	c.bound[id] = extension
	return nil
}

func (c *fakeConversation) SetStep(_, label string) { c.steps = append(c.steps, label) }

func (c *fakeConversation) ClearStep(string) { c.steps = append(c.steps, "") }

// fakeExtensions is the extension manager the workshop checks and installs through.
type fakeExtensions struct {
	existing   map[string]api.Manifest
	methods    []string
	checkErr   error
	installErr error
	installs   []string
	files      map[string]map[string][]byte
}

func newFakeExtensions() *fakeExtensions {
	return &fakeExtensions{existing: map[string]api.Manifest{}, methods: []string{"SearchItems"}, files: map[string]map[string][]byte{}}
}

func (e *fakeExtensions) Definition(id string) (api.Manifest, string, error) {
	manifest, ok := e.existing[id]
	if !ok {
		return api.Manifest{}, "", errors.New("no such extension")
	}
	return manifest, "", nil
}

func (e *fakeExtensions) Check(string, map[string][]byte) ([]string, error) {
	return e.methods, e.checkErr
}

func (e *fakeExtensions) Install(id string, files map[string][]byte, _ bool) error {
	if e.installErr != nil {
		return e.installErr
	}
	e.installs = append(e.installs, id)
	e.files[id] = files
	return nil
}

// finished is the outcome of a turn that ended with its answer.
func finished() chat.Outcome {
	return chat.Outcome{Ending: chat.EndFinished}
}

// builderChat is a builder chat with the id, bound to no extension.
func builderChat(id string) chat.Chat {
	return chat.Chat{ID: id, Kind: chat.KindBuilder, Tier: chat.TierSmart}
}
