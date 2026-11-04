package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// page is the state of one open chat: which chat it shows, the scroll of its transcript, the composer's text and the
// input the composer draws, which the palette focuses.
type page struct {
	deps        Deps
	id          string
	list        ui.ListState
	draft       string
	input       ui.Handle
	attachments []chat.Image
	host        search.Host
}

// ChatView is the page of the chat with id. Its body is the transcript and its composer; the palette has no search
// field of its own here.
func ChatView(deps Deps, id string) *palette.View {
	return newPage(deps, id).view()
}

// newPage is the state of a chat page, before the palette shows it.
func newPage(deps Deps, id string) *page {
	p := &page{deps: deps, id: id}
	p.list.FollowEnd = true
	return p
}

// view is the palette view of the page: the transcript and composer body, its action menu and its footer.
func (p *page) view() *palette.View {
	return &palette.View{
		Title:        "AI Chat",
		TitleFn:      p.title,
		Body:         p.body,
		Input:        &p.input,
		Actions:      p.chatActions,
		Primary:      p.primaryKey,
		SurvivesHide: true,
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

// primaryKey is the footer's primary action: the answer a waiting tool call takes, Stop while a reply streams, or Send.
func (p *page) primaryKey() *design.KeyHint {
	record, _ := p.deps.Service.Chat(p.id)
	switch {
	case chat.AwaitingAnswer(record):
		return &design.KeyHint{Title: "Allow", Key: palette.KeyLabel("↵")}
	case p.deps.Service.Busy(p.id):
		return &design.KeyHint{Title: "Stop"}
	}
	return &design.KeyHint{Title: "Send", Key: palette.KeyLabel("↵")}
}
