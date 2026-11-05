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
// on the UI thread; Stop is nil when the view offers no stop; ChooseTier is nil when the view has no tier picker.
// Actions are the view's action panel rows. Placeholder is the composer's prompt.
type ExtensionChatSpec struct {
	Title       string
	Messages    []chat.Message
	Placeholder string
	Tier        chat.Tier
	Send        func(h search.Host, text string)
	Stop        func(h search.Host)
	ChooseTier  func(h search.Host, tier chat.Tier)
	Actions     func() []search.Action
}

// ExtensionChat draws an extension's chat view with the host's transcript, reasoning and tool rows, and composer. It
// keeps what the user opened and what they typed between frames, so the owner replaces its spec on every render.
type ExtensionChat struct {
	spec  ExtensionChatSpec
	page  *page
	draft string
	input ui.Handle
	host  search.Host
}

// NewExtensionChat returns the renderer. reply draws a reply's Markdown and now reads the clock for the progress rows.
func NewExtensionChat(reply func(c *ui.Context, src string, streaming bool), now func() time.Time) *ExtensionChat {
	return &ExtensionChat{page: newPage(Deps{Reply: reply, Now: now}, "")}
}

// SetSpec replaces what the view shows with spec, as the owner does after each render.
func (x *ExtensionChat) SetSpec(spec ExtensionChatSpec) {
	x.spec = spec
}

// View is the palette page that draws the chat: the transcript above the composer, with the view's actions.
func (x *ExtensionChat) View() *palette.View {
	return &palette.View{
		Title:        x.spec.Title,
		Body:         x.body,
		Input:        &x.input,
		Actions:      x.spec.Actions,
		Primary:      x.primaryKey,
		SurvivesHide: true,
	}
}

// body draws the transcript and the composer on the pane's own surface.
func (x *ExtensionChat) body(c *ui.Context, h search.Host) {
	x.host = h
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

// composer is the card under the transcript: the palette's text input, the tier picker and the send or stop control.
func (x *ExtensionChat) composer(c *ui.Context, k *design.TokenSet) {
	placeholder := x.spec.Placeholder
	if placeholder == "" {
		placeholder = "Message AI"
	}
	ui.Column(c).Radius(8).Background(k.Surface).Border(1, k.EdgeStrong).InsetShadow(0, 1, 0, 0, k.Hi).Children(func() {
		ui.TextAreaBase(c, &x.draft).Bind(&x.input).Lines(1, 6).Placeholder(placeholder).
			PlaceholderColor(k.Faint).FontSize(13).TextColor(k.Strong).Padding(10, 12, 4, 12).
			OnTextKey(x.composerKey)
		ui.Row(c).Padding(0, 6, 6, 6).AlignItems(ui.Center).Gap(4).Children(func() {
			x.tierPicker(c)
			ui.Spacer(c)
			x.sendOrStop(c, k)
		})
	})
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

// send hands the typed text to the view. Empty text is not sent, except that a waiting tool is allowed with it.
func (x *ExtensionChat) send(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || x.spec.Send == nil || x.streaming() {
		return false
	}
	x.spec.Send(x.host, text)
	x.draft = ""
	return true
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
	empty := strings.TrimSpace(x.draft) == ""
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

// primaryKey is the footer's primary action: Stop while a reply streams, otherwise Send.
func (x *ExtensionChat) primaryKey() *design.KeyHint {
	if x.streaming() && x.spec.Stop != nil {
		return &design.KeyHint{Title: "Stop"}
	}
	return &design.KeyHint{Title: "Send", Key: palette.KeyLabel("↵")}
}
