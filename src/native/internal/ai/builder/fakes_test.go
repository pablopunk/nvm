package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeConversation serves the chats the workshop reads, records the bindings and the steps it shows. An empty label in
// steps is a step that ended.
type fakeConversation struct {
	chats  map[string]chat.Chat
	bound  map[string]string
	steps  []string
	active map[string]*chat.BuilderTurn
}

func newFakeConversation(chats ...chat.Chat) *fakeConversation {
	c := &fakeConversation{chats: map[string]chat.Chat{}, bound: map[string]string{}, active: map[string]*chat.BuilderTurn{}}
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

func (c *fakeConversation) ReserveExtension(id, target string, turn *chat.BuilderTurn) error {
	cchat, ok := c.chats[id]
	if !ok || turn == nil || c.active[id] != turn || cchat.Kind != chat.KindBuilder {
		return chat.ErrNotFound
	}
	if cchat.Extension != "" && cchat.Extension != target {
		return chat.ErrExtensionHasChat
	}
	for _, owner := range c.Owners(target) {
		if owner.ID != id {
			return chat.ErrExtensionHasChat
		}
	}
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
	catalog    []Installed
	generation uint64
}

func (e *fakeExtensions) Catalog() []Installed { return e.catalog }

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

type fakeEditRevision struct {
	owner      *fakeExtensions
	id         string
	files      map[string][]byte
	generation uint64
}

func (e *fakeExtensions) EditSnapshot(id string) (map[string][]byte, any, error) {
	files, found := e.files[id]
	if !found {
		if manifest, ok := e.existing[id]; ok {
			data, _ := json.Marshal(manifest)
			files = map[string][]byte{"extension.json": data, "main.go": []byte("package main")}
		}
	}
	copied := cloneFakeFiles(files)
	return cloneFakeFiles(copied), &fakeEditRevision{owner: e, id: id, files: copied, generation: e.generation}, nil
}

func cloneFakeFiles(files map[string][]byte) map[string][]byte {
	if files == nil {
		return nil
	}
	out := make(map[string][]byte, len(files))
	for name, data := range files {
		out[name] = bytes.Clone(data)
	}
	return out
}

func (e *fakeExtensions) InstallEdit(ctx context.Context, id string, files map[string][]byte, _ bool, revision any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	token, ok := revision.(*fakeEditRevision)
	if !ok || token.owner != e || token.id != id || token.generation != e.generation {
		return errors.New("edit conflict")
	}
	current, _, err := e.EditSnapshot(id)
	if err != nil {
		return err
	}
	if len(current) != len(token.files) || (current == nil) != (token.files == nil) {
		return errors.New("edit conflict")
	}
	for name, data := range current {
		if !bytes.Equal(data, token.files[name]) {
			return errors.New("edit conflict")
		}
	}
	if e.installErr != nil {
		return e.installErr
	}
	e.generation++
	e.installs = append(e.installs, id)
	e.files[id] = cloneFakeFiles(files)
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
