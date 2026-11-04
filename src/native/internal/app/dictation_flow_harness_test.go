package app

import (
	"context"
	"sync"

	dict "github.com/pablopunk/nvm/native/internal/dictation"
	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// fakeDictationAccount is a signed-in or signed-out account that records the refusals the flow notes on it.
type fakeDictationAccount struct {
	signedIn    bool
	transcriber dict.Transcriber
	notes       []error
}

func (a *fakeDictationAccount) SignedIn() bool                { return a.signedIn }
func (a *fakeDictationAccount) Transcriber() dict.Transcriber { return a.transcriber }
func (a *fakeDictationAccount) NoteError(err error)           { a.notes = append(a.notes, err) }

// fakeDictationPanel records every indicator change in order. It is safe to read while a capture goroutine reports
// levels, which is what the real panel's UI thread does too.
type fakeDictationPanel struct {
	mu     sync.Mutex
	events []string
	states []palindicator.State
}

func (p *fakeDictationPanel) Show(state palindicator.State) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.states = append(p.states, state)
	p.events = append(p.events, "show:"+modeName(state.Mode))
}

func (p *fakeDictationPanel) Hide() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, "hide")
}

func (p *fakeDictationPanel) eventList() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.events...)
}

func (p *fakeDictationPanel) stateList() []palindicator.State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]palindicator.State(nil), p.states...)
}

func (p *fakeDictationPanel) last() palindicator.State {
	states := p.stateList()
	if len(states) == 0 {
		return palindicator.State{}
	}
	return states[len(states)-1]
}

func modeName(mode palindicator.Mode) string {
	return []string{"listening", "waiting", "transcribing", "no-speech", "failed", "cleaning"}[mode]
}

// fakeDictationTranscriber answers each segment with the next scripted text, or fails with err.
type fakeDictationTranscriber struct {
	texts    []string
	err      error
	segments []dict.Segment
	released []bool
}

func (t *fakeDictationTranscriber) Prepare(context.Context, string) error { return nil }

func (t *fakeDictationTranscriber) Transcribe(_ context.Context, segment dict.Segment) (string, error) {
	t.segments = append(t.segments, segment)
	if t.err != nil {
		return "", t.err
	}
	text := t.texts[0]
	t.texts = t.texts[1:]
	return text, nil
}

func (t *fakeDictationTranscriber) CancelPreparation(_ context.Context, _ string, userCancelled bool) error {
	t.released = append(t.released, userCancelled)
	return nil
}

// pasteRecord is what the paste port saw: how many palette hides had happened, what the clipboard held, and the app
// the paste was aimed at.
type pasteRecord struct {
	hidden    int
	concealed bool
	text      string
	target    string
}

// orderedPaster records each paste with the state the palette and clipboard were in when the keys were sent.
type orderedPaster struct {
	harness *dictationHarness
}

func (p orderedPaster) PasteIntoFrontmost(target string) error {
	h := p.harness
	formats, _ := h.set.Clipboard.Formats()
	text, _ := h.set.Clipboard.ReadText()
	h.pastes = append(h.pastes, pasteRecord{hidden: h.hidden, concealed: hasConcealed(formats), text: text, target: target})
	return h.pasteErr
}

func (p orderedPaster) FrontmostAppID() (string, error) {
	return p.harness.frontmost, p.harness.frontmostErr
}

func hasConcealed(formats []platform.ClipboardFormat) bool {
	for _, format := range formats {
		if format == platform.FormatConcealed {
			return true
		}
	}
	return false
}
