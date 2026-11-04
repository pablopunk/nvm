package app

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/palette/aicommands"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// attachAICommands builds the AI commands over the app's ports and chat service. It runs after attachChats; without
// the chat store the commands are not offered.
func (p *App) attachAICommands() {
	if p.chats == nil {
		return
	}
	p.aiCommands = commands.NewService(commands.Options{
		Clock:        searchClock,
		Clipboard:    p.ports.Clipboard,
		SelectedText: p.ports.SelectedText,
		Paster:       p.ports.Paster,
		Focus:        p.focusOption(),
		Chats:        p.chats,
		Client:       p.backendClient,
		Connected:    p.signedIn,
		Surface:      aiSurface{app: p},
		Dispatch:     p.onUI,
		Background:   supervisedCommand,
		After:        p.commandAfter,
	})
}

// supervisedCommand runs an AI command's background work on a supervised goroutine, so a panic is reported.
func supervisedCommand(name string, fn func()) {
	safego.Go(name, func(context.Context) { fn() })
}

// commandAfter runs fn on the UI thread after d.
func (p *App) commandAfter(d time.Duration, fn func()) {
	time.AfterFunc(d, func() { p.redraw(fn) })
}

// aiSurface is what the AI commands draw on: the indicator panel, the palette's result page and its toasts.
type aiSurface struct {
	app *App
}

// ShowStatus shows the indicator with status.
func (s aiSurface) ShowStatus(status commands.Status) {
	s.app.aiPanelFor().show(status)
}

// HideStatus closes the indicator.
func (s aiSurface) HideStatus() {
	s.app.aiPanelFor().hide()
}

// Present opens the result page on the palette.
func (s aiSurface) Present(result commands.Result) {
	s.app.Open(aicommands.ResultView(result))
}

// Toast shows a one-line message on the palette.
func (s aiSurface) Toast(message string) {
	s.app.Toast(message)
}

// DismissPalette hides the palette. A stale hide signal is dropped first, so the paste that follows waits for this hide.
func (s aiSurface) DismissPalette() {
	s.app.discardPaletteHidden()
	s.app.Hide()
}

// discardPaletteHidden forgets a hide the paste port has not consumed yet.
func (p *App) discardPaletteHidden() {
	select {
	case <-p.paletteHidden:
	default:
	}
}

// aiCommandRows are the AI commands in the root list, or none when the chat store did not open. Fix and triage need a
// selection the OS can copy; classification reads the clipboard and is always offered.
func (p *App) aiCommandRows() []command {
	if p.aiCommands == nil {
		return nil
	}
	selection := p.ports.Capabilities.Supported(platform.CapabilitySelectionCopy)
	return []command{
		{Title: commands.FixTitle, Keywords: "fix grammar correct proofread spelling selected text ai", Icon: "sparkles", Subtitle: "Correct grammar, spelling, and punctuation", Hidden: !selection,
			Run: func(h search.Host) { p.runSelectionCommand(h, p.aiCommands.Fix) }},
		{Title: commands.TriageTitle, Keywords: "triage classify decide reply urgency selected text ai", Icon: "sparkles", Subtitle: "Reply needed, category and urgency in one call", Hidden: !selection,
			Run: func(h search.Host) { p.runSelectionCommand(h, p.aiCommands.Triage) }},
		{Title: commands.ClassifyTitle, Keywords: "classify image picture screenshot photograph ai", Icon: "image", Subtitle: "Copy an image, then identify its category",
			Run: func(h search.Host) { h.Hide(); p.aiCommands.ClassifyClipboardImage() }},
	}
}

// runSelectionCommand hides the palette, which gives the keyboard back to the app the selection is in, and starts the
// command on that app. The app is read before the hide, while the palette still knows it.
func (p *App) runSelectionCommand(h search.Host, start func(source platform.FocusTarget)) {
	source, _ := p.Summoned()
	h.Hide()
	start(source)
}
