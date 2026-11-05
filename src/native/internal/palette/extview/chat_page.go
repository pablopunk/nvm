package extview

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/aichat"
	"github.com/pablopunk/nvm/native/internal/search"
)

// chatPage draws a chat view with the host's chat renderer. Send, Stop and the tier picker run the view's handlers, and
// the view's reply replaces what is shown, as a form's submit does.
func (s *Session) chatPage() *palette.View {
	if s.chatRenderer == nil {
		s.chatRenderer = aichat.NewExtensionChat(s.replyRenderer(), s.now)
		s.chatRenderer.SpecFrom(s.chatSpec)
	}
	s.chatRenderer.SetSpec(s.chatSpec())
	return s.chatRenderer.View()
}

// now reads the session clock, the zero time when the host gave none.
func (s *Session) now() time.Time {
	if s.deps.Clock == nil {
		return time.Time{}
	}
	return s.deps.Clock.Now()
}

// replyRenderer is the Markdown drawer for replies, or plain text when the host gave none.
func (s *Session) replyRenderer() func(c *ui.Context, src string, streaming bool) {
	if s.deps.Reply != nil {
		return s.deps.Reply
	}
	return func(c *ui.Context, src string, _ bool) { ui.Text(c, src).FontSize(13) }
}

// chatSpec turns the view's chat into what the renderer draws, with its handlers bound to this session.
func (s *Session) chatSpec() aichat.ExtensionChatSpec {
	spec := s.view.Chat
	out := aichat.ExtensionChatSpec{
		Title:       s.view.Title,
		Messages:    chatMessages(spec.Messages, s.now()),
		Placeholder: spec.Placeholder,
		Tier:        chatTier(spec.Tier),
		Actions:     s.pageActions,
		Attach:      spec.Attach,
		PickImages:  s.deps.PickImages,
	}
	if spec.Side != nil {
		out.Side = s.chatSide
	}
	out.Host = s.attachHost
	if spec.Submit != nil {
		submit := *spec.Submit
		out.Send = func(h search.Host, text string, images []chat.Image) {
			s.submit(h, api.Invocation{Ref: submit, Form: api.FormValues{"message": {Kind: api.FormText, Text: text}}, Images: apiChatImages(images)}, nil)
		}
	}
	if spec.Stop != nil {
		stop := *spec.Stop
		out.Stop = func(h search.Host) { s.submit(h, api.Invocation{Ref: stop}, nil) }
	}
	if spec.OnTier != nil {
		onTier := *spec.OnTier
		out.ChooseTier = func(h search.Host, tier chat.Tier) {
			ref := onTier
			ref.Args = map[string]string{"tier": string(tier)}
			s.submit(h, api.Invocation{Ref: ref}, nil)
		}
	}
	return out
}

// chatSide is the side page of a chat view: a palette page over the view's side, built once and kept, so its search and
// cursor survive the renders. Each render hands the page the side the owner returned.
func (s *Session) chatSide() *palette.View {
	side := *s.view.Chat.Side
	if s.chatSideSession == nil {
		s.chatSideSession = NewSession(s.deps, s.extID, side)
		s.chatSidePage = s.chatSideSession.Palette()
		return s.chatSidePage
	}
	s.chatSideSession.Replace(side)
	return s.chatSidePage
}

// apiChatImages maps the pictures a user attached to a message to the API's data URLs, which a submit carries.
func apiChatImages(images []chat.Image) []api.ChatImage {
	out := make([]api.ChatImage, 0, len(images))
	for _, image := range images {
		out = append(out, api.ChatImage{Title: image.Name, DataURL: "data:" + image.MimeType + ";base64," + image.Data})
	}
	return out
}

// chatMessages maps the view's messages to transcript entries. A user turn keeps its pictures, a tool row keeps its
// outcome, and a system line shows as a note.
func chatMessages(messages []api.ChatMessage, now time.Time) []chat.Message {
	out := make([]chat.Message, 0, len(messages))
	for _, m := range messages {
		out = append(out, chatMessage(m, now))
	}
	return out
}

// chatMessage maps one view message to a transcript entry.
func chatMessage(m api.ChatMessage, now time.Time) chat.Message {
	switch {
	case m.Tool != nil:
		return chat.Message{Author: chat.AuthorTool, Status: toolStatus(m.Tool.Status), At: now, Tool: &chat.ToolRun{
			Name: m.Tool.Name, Label: m.Tool.Name, Outcome: m.Tool.Result,
		}}
	case m.Role == "user":
		return chat.Message{Author: chat.AuthorUser, Text: m.Content, Images: chatImages(m.Images), At: now, Status: chat.StatusComplete}
	case m.Role == "system":
		return chat.Message{Author: chat.AuthorNotice, Notice: &chat.Notice{Title: noticeTitle(m.Title), Message: m.Content}, At: now}
	}
	status := chat.StatusComplete
	if m.Streaming {
		status = chat.StatusStreaming
	}
	return chat.Message{Author: chat.AuthorAssistant, Text: m.Content, Reasoning: m.Thinking, Status: status, At: now}
}

// toolStatus maps a tool row's status word to the transcript's status.
func toolStatus(status string) chat.Status {
	switch status {
	case "running":
		return chat.StatusRunning
	case "waiting":
		return chat.StatusAwaiting
	case "failed":
		return chat.StatusFailed
	}
	return chat.StatusComplete
}

// chatImages maps the pictures of a user turn, which arrive as data URLs.
func chatImages(images []api.ChatImage) []chat.Image {
	out := make([]chat.Image, 0, len(images))
	for _, image := range images {
		mime, data := splitDataURL(image.DataURL)
		out = append(out, chat.Image{Name: image.Title, MimeType: mime, Data: data})
	}
	return out
}

// chatTier maps the view's tier to the chat tier; an empty tier is smart, as the API says.
func chatTier(tier api.ModelTier) chat.Tier {
	if tier == api.ModelFast {
		return chat.TierFast
	}
	return chat.TierSmart
}

// splitDataURL returns the media type and the base64 payload of a data URL. A value that is not one has no type and
// keeps its text as the payload, so the picture simply does not draw.
func splitDataURL(dataURL string) (mime, data string) {
	const prefix = "data:"
	const marker = ";base64,"
	rest, ok := strings.CutPrefix(dataURL, prefix)
	if !ok {
		return "", dataURL
	}
	mime, data, ok = strings.Cut(rest, marker)
	if !ok {
		return "", dataURL
	}
	return mime, data
}

// attachHost gives a session that was built without a host one, the first time its chat page draws, and starts its
// background work, such as the refresh that fills a loading view in. A session opened with Open already has its host.
func (s *Session) attachHost(h search.Host) {
	if s.host != nil {
		return
	}
	s.host = h
	s.start()
}

// noticeTitle is the title of a system row: the title the message names, or Note.
func noticeTitle(title string) string {
	if title == "" {
		return "Note"
	}
	return title
}
