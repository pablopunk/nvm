package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// hint is the right footer of a chat page: the keys the page answers beyond typing. awaitingHint replaces it while a
// tool call waits for the user's answer.
const (
	hint         = "⌘R ask again · ⌘⌥C copy reply · ⌘⇧M model"
	awaitingHint = "↵ allow · ⌘D refuse · Esc stop"
)

// page is the state of one open chat: which chat it shows, the scroll of its transcript and the host it last drew
// with, so keys and submits can answer on the palette.
type page struct {
	deps Deps
	id   string
	list ui.ListState
	host search.Host
}

// ChatView is the page of the chat with id. Its body is the transcript; the palette input is the composer.
func ChatView(deps Deps, id string) *palette.View {
	p := &page{deps: deps, id: id}
	p.list.FollowEnd = true
	return &palette.View{
		Title:       "AI Chat",
		TitleFn:     p.title,
		Placeholder: placeholderFor(deps.Service, id),
		Body:        p.body,
		OnSubmit:    p.submit,
		OnKey:       p.key,
		Hint:        hint,
		HintFn:      p.footerHint,
	}
}

// title is the chat's title, or the default for its kind.
func (p *page) title() string {
	record, ok := p.deps.Service.Chat(p.id)
	switch {
	case ok && record.Title != "":
		return record.Title
	case ok && record.Kind == chat.KindBuilder:
		return "Create Extension"
	}
	return "AI Chat"
}

// placeholderFor is the input's prompt for the chat with id: the builder asks for an extension, a conversation
// asks for a message.
func placeholderFor(service *chat.Service, id string) string {
	if record, ok := service.Chat(id); ok && record.Kind == chat.KindBuilder {
		return "Describe the extension you want"
	}
	return "Message AI"
}

// submit sends the typed text as the next message. Enter on an empty input allows a tool call that waits for the user.
// A refusal is said on the palette, not in the transcript.
func (p *page) submit(text string) {
	if text == "" && p.deps.Service.Awaiting(p.id) {
		p.report(p.deps.Service.Decide(p.id, true))
		return
	}
	p.report(p.deps.Service.Send(p.id, text))
}

// footerHint is the right footer of the page: it names the answers a waiting tool call takes, as the transcript shows.
func (p *page) footerHint() string {
	if record, ok := p.deps.Service.Chat(p.id); ok && chat.AwaitingAnswer(record) {
		return awaitingHint
	}
	return hint
}

// report toasts a refusal the transcript does not show. A nil error is silent.
func (p *page) report(err error) {
	if err == nil || p.host == nil {
		return
	}
	p.host.Toast(refusalCopy(err))
}
