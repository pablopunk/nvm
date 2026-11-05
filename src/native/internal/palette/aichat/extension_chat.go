package aichat

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ExtensionChatSpec is what an extension's chat view shows. Messages are the transcript; Send receives the typed text
// and the pictures attached to it on the UI thread; Stop is nil when the view offers no stop; ChooseTier is nil when the
// view has no tier picker. Attach with PickImages shows the picture control. Side returns the page drawn beside the
// transcript, or nil for none. Actions are the view's action panel rows. Placeholder is the composer's prompt.
type ExtensionChatSpec struct {
	Title       string
	Messages    []chat.Message
	Placeholder string
	Tier        chat.Tier
	Attach      bool
	PickImages  func(done func([]chat.Image))
	Send        func(h search.Host, text string, images []chat.Image)
	Stop        func(h search.Host)
	ChooseTier  func(h search.Host, tier chat.Tier)
	Actions     func() []search.Action
	Side        func() *palette.View
	// Host receives the palette host each frame the transcript draws, so the owner can start work that needs it.
	Host func(h search.Host)
}

// ExtensionChat draws an extension's chat view with the host's transcript, reasoning and tool rows, and composer. It
// keeps what the user opened, typed and attached between frames, so the owner replaces its spec on every render.
type ExtensionChat struct {
	spec          ExtensionChatSpec
	page          *page
	draft         string
	images        []chat.Image
	input         ui.Handle
	host          search.Host
	side          *palette.Side
	sideRoot      *palette.View
	previewHidden bool
	specFrom      func() ExtensionChatSpec
}

// NewExtensionChat returns the renderer. reply draws a reply's Markdown and now reads the clock for the progress rows.
func NewExtensionChat(reply func(c *ui.Context, src string, streaming bool), now func() time.Time) *ExtensionChat {
	return &ExtensionChat{page: newPage(renderDeps{Reply: reply, Now: now})}
}

// SetSpec replaces what the view shows with spec, as the owner does after each render.
func (x *ExtensionChat) SetSpec(spec ExtensionChatSpec) {
	x.spec = spec
}

// SpecFrom makes the view read its spec from fn at every frame, so a view the owner changed between renders, such as a
// refreshed transcript, draws its current state without the owner rebuilding the page.
func (x *ExtensionChat) SpecFrom(fn func() ExtensionChatSpec) {
	x.specFrom = fn
}

// View is the palette page that draws the chat: the transcript above the composer, with the view's actions and, when
// the spec has one, the side page beside it.
func (x *ExtensionChat) View() *palette.View {
	return &palette.View{
		Title:        x.spec.Title,
		Body:         x.body,
		Input:        &x.input,
		Actions:      x.actions,
		Primary:      x.primaryKey,
		Side:         x.sideFor,
		SurvivesHide: true,
	}
}

// body draws the transcript and the composer on the pane's own surface.
func (x *ExtensionChat) body(c *ui.Context, h search.Host) {
	x.host = h
	if x.specFrom != nil {
		x.spec = x.specFrom()
	}
	if x.spec.Host != nil {
		x.spec.Host(h)
	}
	k := design.Tokens(c)
	ui.Column(c).Grow(1).FillHeight().MinHeight(0).Padding(8, 16, 12, 16).Children(func() {
		x.transcript(c)
		x.composer(c, k)
	})
}

// transcript is the scrolling list of messages, which follows the newest one. An empty chat is blank.
func (x *ExtensionChat) transcript(c *ui.Context) {
	messages := x.spec.Messages
	if len(messages) == 0 {
		ui.Column(c).Grow(1)
		return
	}
	ui.List(c, &x.page.list, len(messages), func(i int) {
		ui.Column(c).Padding(6, 4).Children(func() {
			renderMessage(c, x.page, i, messages[i], i == len(messages)-1)
		})
	}).Grow(1)
}

// composer is the card under the transcript: the pictures the next message carries, the palette's text input, the
// tier picker, the picture control and the send or stop control.
func (x *ExtensionChat) composer(c *ui.Context, k *design.TokenSet) {
	placeholder := x.spec.Placeholder
	if placeholder == "" {
		placeholder = "Message AI"
	}
	ui.Column(c).Radius(8).Background(k.Surface).Border(1, k.EdgeStrong).InsetShadow(0, 1, 0, 0, k.Hi).Children(func() {
		if len(x.images) > 0 {
			attachmentsRow(c, x.images, x.removeImage)
		}
		ui.TextAreaBase(c, &x.draft).Bind(&x.input).Lines(1, 6).Placeholder(placeholder).
			PlaceholderColor(k.Faint).FontSize(13).TextColor(k.Strong).Padding(10, 12, 4, 12).
			OnTextKey(x.composerKey)
		ui.Row(c).Padding(0, 6, 6, 6).AlignItems(ui.Center).Gap(4).Children(func() {
			x.tierPicker(c)
			if x.attaches() {
				attachButton(c, x.spec.PickImages, x.addImages)
			}
			ui.Spacer(c)
			x.sendOrStop(c, k)
		})
	})
}

// attaches reports whether the composer offers pictures: the view asked for them and the host can read them.
func (x *ExtensionChat) attaches() bool {
	return x.spec.Attach && x.spec.PickImages != nil
}

// addImages attaches pictures to the next message, as many as the limits allow, and says which limit refused the rest.
// The composer takes the keyboard back, as the chooser that picked them took it away.
func (x *ExtensionChat) addImages(incoming []chat.Image) {
	kept, problem := chat.AddImages(x.images, incoming)
	x.images = kept
	x.input.Focus()
	if problem != "" && x.host != nil {
		x.host.Toast(problem)
	}
}

// removeImage takes the picture at index off the next message.
func (x *ExtensionChat) removeImage(index int) {
	x.images = append(x.images[:index:index], x.images[index+1:]...)
}

// composerKey answers the composer's text: Enter sends what is typed, Shift+Enter breaks the line.
func (x *ExtensionChat) composerKey(key ui.TextKey) bool {
	if key.Key != ui.KeyEnter || key.Mods != 0 {
		return false
	}
	if x.send(key.Text()) {
		key.SetText("")
	}
	return true
}

// send hands the typed text and the attached pictures to the view. A message needs text or a picture, or an empty Enter
// that allows the tool call waiting for the user.
func (x *ExtensionChat) send(text string) bool {
	text = strings.TrimSpace(text)
	if !x.canSend(text) || x.spec.Send == nil || x.streaming() {
		return false
	}
	x.spec.Send(x.host, text, x.images)
	x.draft, x.images = "", nil
	return true
}

func (x *ExtensionChat) canSend(text string) bool {
	return text != "" || len(x.images) > 0 || x.awaitingAnswer()
}

func (x *ExtensionChat) awaitingAnswer() bool {
	return chat.MessagesAwaitAnswer(x.spec.Messages)
}

// streaming reports whether the newest message is still arriving, which is when the view offers Stop.
func (x *ExtensionChat) streaming() bool {
	for _, m := range x.spec.Messages {
		if m.Status == chat.StatusStreaming {
			return true
		}
	}
	return false
}

// sendOrStop is the control at the end of the composer: Stop while a reply streams, otherwise send.
func (x *ExtensionChat) sendOrStop(c *ui.Context, k *design.TokenSet) {
	if x.streaming() && x.spec.Stop != nil {
		stop := ui.ButtonBase(c).Padding(4, 10).Radius(6).Background(k.Interactive)
		stop.Children(func() {
			ui.Text(c, "Stop").FontSize(12).FontWeight(600).TextColor(k.Danger)
		})
		if stop.Clicked() {
			x.spec.Stop(x.host)
		}
		return
	}
	empty := strings.TrimSpace(x.draft) == "" && len(x.images) == 0
	send := ui.ButtonBase(c).Padding(4).Radius(14).Background(k.AccentFill).Disabled(empty)
	if empty {
		send.Opacity(0.35)
	}
	send.Children(func() {
		ui.Icon(c, design.Icon("corner-down-left")).FontSize(14).TextColor(k.AccentContrast)
	})
	if send.Clicked() {
		x.send(x.draft)
	}
}

// tierPicker names the tier the next reply uses. It is shown only when the view handles tier changes, and a press opens
// the native menu of the tiers. The backend's model and route names are never shown.
func (x *ExtensionChat) tierPicker(c *ui.Context) {
	if x.spec.ChooseTier == nil {
		return
	}
	chosen := design.Select(c, design.SelectData{
		Options:  tierTitles(),
		Selected: slices.Index(tierOrder, x.spec.Tier),
		Disabled: x.streaming(),
	})
	if chosen >= 0 {
		x.spec.ChooseTier(x.host, tierOrder[chosen])
	}
}

// actions are the view's action rows, the reasoning toggle before the tier row, then the preview toggle and focus rows
// when the view has a side page.
func (x *ExtensionChat) actions() []search.Action {
	var out []search.Action
	if x.spec.Actions != nil {
		out = x.spec.Actions()
	}
	out = withThinkingRow(out, x.thinkingAction())
	if x.spec.Side == nil || x.spec.Side() == nil {
		return out
	}
	out = append(out, x.previewAction())
	if !x.previewHidden {
		out = append(out, x.focusPreviewAction())
	}
	return out
}

// previewAction shows or hides the side page beside the transcript, as ⌃⇧P does on the AI Chats page.
func (x *ExtensionChat) previewAction() search.Action {
	title := "Show Preview"
	if !x.previewHidden {
		title = "Hide Preview"
	}
	return search.Action{Title: title, Label: palette.KeyLabel("⌃⇧P"), Key: ui.KeyP, Mods: palette.Mod | ui.Shift, Dismiss: search.DismissNever,
		Run: func(h search.Host) {
			x.previewHidden = !x.previewHidden
			h.Refresh()
		}}
}

// focusPreviewAction gives the keyboard to the side page.
func (x *ExtensionChat) focusPreviewAction() search.Action {
	return search.Action{Title: "Focus Preview", Dismiss: search.DismissNever, Run: func(search.Host) {
		x.side.Focus()
	}}
}

// sideFor is the side page beside the transcript, or nil while the user hides it or the view has none. The palette
// page of the side is kept between frames, so its search and cursor survive, and it follows the view the owner returns.
func (x *ExtensionChat) sideFor() *palette.Side {
	if x.spec.Side == nil || x.previewHidden {
		return nil
	}
	view := x.spec.Side()
	if view == nil {
		return nil
	}
	if x.side == nil {
		x.side = palette.NewSide(view)
		x.input.Focus()
	} else if view != x.sideRoot {
		x.side.SetRoot(view)
	}
	x.sideRoot = view
	return x.side
}

// primaryKey is the footer's primary action: Stop while a reply streams, otherwise Send.
func (x *ExtensionChat) primaryKey() *design.KeyHint {
	if x.streaming() && x.spec.Stop != nil {
		return &design.KeyHint{Title: "Stop"}
	}
	return &design.KeyHint{Title: "Send", Key: palette.KeyLabel("↵")}
}
