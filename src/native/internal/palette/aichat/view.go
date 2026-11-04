package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// hint is the right footer of a chat page: the keys the page answers beyond typing. awaitingHint replaces it while a
// tool call waits for the user's answer, and reviewHint while a builder chat has an extension to review.
const (
	hint         = "⌘R ask again · ⌘⌥C copy reply · ⌘⇧M model"
	awaitingHint = "↵ allow · ⌘D refuse · Esc stop"
	reviewHint   = "⌘E review extension · ⌘R ask again · ⌘⇧M model"
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
		Title:        "AI Chat",
		TitleFn:      p.title,
		Placeholder:  placeholderFor(deps.Service, id),
		Body:         p.body,
		OnSubmit:     p.submit,
		OnKey:        p.key,
		OnEscape:     p.escape,
		SurvivesHide: true,
		Hint:         hint,
		HintFn:       p.footerHint,
	}
}

// title is the chat's title, or the default for its kind: a builder chat that changes an extension is an edit.
func (p *page) title() string {
	record, ok := p.deps.Service.Chat(p.id)
	switch {
	case ok && record.Title != "":
		return record.Title
	case ok && record.Kind == chat.KindBuilder && record.Extension != "":
		return "Edit Extension"
	case ok && record.Kind == chat.KindBuilder:
		return "Create Extension"
	}
	return "AI Chat"
}

// placeholderFor is the input's prompt for the chat with id: a builder chat asks for an extension or for the change
// to one, and a conversation asks for a message.
func placeholderFor(service *chat.Service, id string) string {
	record, ok := service.Chat(id)
	switch {
	case ok && record.Kind == chat.KindBuilder && record.Extension != "":
		return "Describe the change to this extension"
	case ok && record.Kind == chat.KindBuilder:
		return "Describe the extension you want"
	}
	return "Message AI"
}

// submit sends the typed text as the next message. Enter on an empty input allows a tool call that waits for the user.
// With no account connected, Enter starts the sign-in instead. A refusal is said on the palette, not in the transcript.
func (p *page) submit(text string) {
	if !p.deps.SignedIn() {
		p.startSignIn()
		return
	}
	if text == "" && p.deps.Service.Awaiting(p.id) {
		p.report(p.deps.Service.Decide(p.id, true))
		return
	}
	p.report(p.deps.Service.Send(p.id, text))
}

// footerHint is the right footer of the page: it names the answers a waiting tool call takes, as the transcript shows,
// and the review key of a builder chat that wrote an extension.
func (p *page) footerHint() string {
	record, ok := p.deps.Service.Chat(p.id)
	switch {
	case !p.deps.SignedIn():
		return signInHint
	case ok && chat.AwaitingAnswer(record):
		return awaitingHint
	case ok && record.Extension != "":
		return reviewHint
	}
	return hint
}

// startSignIn begins the sign-in the gate offers and toasts why it could not start.
func (p *page) startSignIn() {
	if err := p.deps.StartSignIn(); err != nil && p.host != nil {
		p.host.Toast(signInRefusal(err))
	}
}

// report toasts a refusal the transcript does not show. A nil error is silent.
func (p *page) report(err error) {
	if err == nil || p.host == nil {
		return
	}
	p.host.Toast(refusalCopy(err))
}
