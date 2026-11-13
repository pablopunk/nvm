package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log"
	"time"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ai/learning"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/store"
)

const builderLearningJobID = "ai.builder.learning-review"
const builderLearningEvent = "ai.builder.learning-requested"
const builderLearningPrompt = `Review the sanitized builder-chat context for durable, generic preferences, workflows or environment rules. Return only JSON {"proposals":[{"operation":"add|update|remove","id":"existing ID for update/remove only","kind":"preference|workflow|environment","text":"rule text","appliesWhen":"optional"}]}. Omit secrets, paths, source, one-off details and tool data. Return an empty proposals array if nothing should change.`

type builderLearningRuntime struct {
	service *learning.Service
	mailbox *learning.Mailbox
}

func (p *App) startBuilderLearning(st *store.Store) {
	if p.userReadOnly {
		return
	}
	doc := store.Doc(st, "builder-learnings.json", 1, func() learning.Document { return learning.Document{} }).WithoutBackups()
	mygo.App.OnQuit(func() {
		p.builderLearningStopped.Store(true)
		if rt := p.builderLearning.Load(); rt != nil {
			rt.mailbox.Close()
		}
	})
	go func() {
		_, err := p.ports.FS.Lstat(doc.Path())
		exists := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Println("builder learnings: storage unavailable")
			return
		}
		key, err := learning.FingerprintKey(p.ports.Keychain, exists, nil)
		if err != nil {
			log.Println("builder learnings: key unavailable")
			return
		}
		rules, err := learning.OpenRules(doc, key)
		if err != nil {
			log.Println("builder learnings: storage unavailable")
			return
		}
		service := learning.NewService(rules, p.askBuilderLearning, newBuilderLearningID, func() {
			p.onUI(func() {
				if p.background != nil {
					p.background.Emit(builderLearningEvent, nil)
				}
			})
		})
		runtime := &builderLearningRuntime{service: service, mailbox: learning.NewMailbox(service)}
		if p.builderLearningStopped.Load() {
			runtime.mailbox.Close()
			return
		}
		p.builderLearning.Store(runtime)
		if p.builderLearningStopped.Load() {
			runtime.mailbox.Close()
		}
	}()
}
func newBuilderLearningID() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

type learningCompletion struct{ client *backend.Client }

func (c learningCompletion) ActiveModel(ctx context.Context, q backend.ActiveModelQuery) (backend.ModelDescriptor, error) {
	return c.client.ActiveModel(ctx, q)
}
func (c learningCompletion) ChatStream(ctx context.Context, req backend.ChatRequest) (chat.EventStream, error) {
	return c.client.ChatStream(ctx, req)
}
func (p *App) askBuilderLearning(ctx context.Context, input string) (string, error) {
	if err := p.checkCompatibility(ctx); err != nil {
		return "", err
	}
	client := p.backendClient()
	if client == nil {
		return "", errors.New("builder learning: account unavailable")
	}
	return chat.CompleteBounded(ctx, learningCompletion{client}, builderLearningPrompt, input, 16<<10)
}
func (p *App) builderLearningJob() jobs.Definition {
	return jobs.Definition{ID: builderLearningJobID, Title: "Review Builder Learnings", Owner: jobs.OwnerHost, Scope: "ai", Triggers: []jobs.Trigger{{Kind: jobs.TriggerEvent, Event: builderLearningEvent}}, Budget: jobs.Budget{Timeout: time.Minute}, Run: func(ctx context.Context, _ jobs.Run) error {
		if rt := p.builderLearning.Load(); rt != nil {
			return rt.service.Run(ctx)
		}
		return nil
	}}
}
func (p *App) builderLearningExit(id string) {
	if p.chats == nil {
		return
	}
	c, ok := p.chats.Chat(id)
	if !ok || c.Kind != chat.KindBuilder {
		return
	}
	if rt := p.builderLearning.Load(); rt != nil {
		rt.mailbox.Exit(id, p.chats.Busy(id))
	}
}
func (p *App) removeBuilderLearning(id string) {
	delete(p.builderLearningSeen, id)
	delete(p.builderLearningEpoch, id)
	for i, entry := range p.builderLearningOrder {
		if entry == id {
			p.builderLearningOrder = append(p.builderLearningOrder[:i], p.builderLearningOrder[i+1:]...)
			break
		}
	}
	if rt := p.builderLearning.Load(); rt != nil {
		rt.mailbox.Remove(id)
	}
}

// builderPromptWithLearning records the first new message boundary without re-reading old transcript history.
func (p *App) builderPromptWithLearning(c chat.Chat) string {
	if p.builderLearningSeen == nil {
		p.builderLearningSeen = map[string]int{}
		p.builderLearningEpoch = map[string]uint64{}
	}
	if _, ok := p.builderLearningSeen[c.ID]; !ok {
		if len(p.builderLearningOrder) >= 32 {
			delete(p.builderLearningSeen, p.builderLearningOrder[0])
			delete(p.builderLearningEpoch, p.builderLearningOrder[0])
			p.builderLearningOrder = p.builderLearningOrder[1:]
		}
		start := len(c.Messages)
		if start > 0 {
			start--
		}
		p.builderLearningSeen[c.ID] = start
		p.builderLearningOrder = append(p.builderLearningOrder, c.ID)
	}
	text := p.builder.SystemPrompt(c)
	if rt := p.builderLearning.Load(); rt != nil {
		p.builderLearningEpoch[c.ID] = rt.service.Epoch()
		if doc, err := rt.service.Snapshot(); err == nil {
			query := ""
			if len(c.Messages) > 0 {
				query = c.Messages[len(c.Messages)-1].Text
			}
			text += "\n\n" + learning.Context(doc, query)
		}
	}
	return text
}

// captureBuilderLearning reads only messages added since this run began; tool payloads and images never leave chat.
func (p *App) captureBuilderLearning(c chat.Chat) {
	start, ok := p.builderLearningSeen[c.ID]
	if !ok {
		return
	}
	if start < 0 || start > len(c.Messages) {
		start = len(c.Messages)
	}
	p.builderLearningSeen[c.ID] = len(c.Messages)
	rt := p.builderLearning.Load()
	if rt == nil {
		return
	}
	messages, outcomes := builderLearningEvents(c.Messages[start:])
	rt.mailbox.CaptureAt(c.ID, p.builderLearningEpoch[c.ID], messages, outcomes)
	rt.mailbox.Settled(c.ID)
}

// builderLearningEvents retains only bounded conversation text and outcome categories.
func builderLearningEvents(source []chat.Message) ([]learning.Message, []string) {
	var messages []learning.Message
	var outcomes []string
	attached := false
	for _, msg := range source {
		if msg.Author == chat.AuthorUser {
			attached = len(msg.Images) > 0
		}
		if (msg.Author == chat.AuthorUser || msg.Author == chat.AuthorAssistant) && !attached && len(msg.Images) == 0 && len(msg.Text) <= 1000 {
			messages = append(messages, learning.Message{Role: string(msg.Author), Text: msg.Text})
			if len(messages) > 20 {
				messages = messages[1:]
			}
		}
		if msg.Author == chat.AuthorTool {
			kind := "failure"
			if msg.Status == chat.StatusComplete {
				kind = "success"
			} else if msg.Status == chat.StatusCancelled {
				kind = "cancelled"
			}
			outcomes = append(outcomes, kind)
			if len(outcomes) > 20 {
				outcomes = outcomes[1:]
			}
		}
	}
	return messages, outcomes
}
