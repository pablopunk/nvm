package commands

import (
	"context"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/features/selection"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Titles of the commands, as the palette and the indicator name them.
const TriageTitle = "Triage Selected Text with AI"

// Dismissal delays: a result the user should read stays longer than a failure's one-line copy would need.
const (
	triageDismiss  = 6 * time.Second
	failureDismiss = 4 * time.Second
	triageTimeout  = 30 * time.Second
)

// Surface is what the commands draw on. The indicator is shown while a command works, and a result or a failure is
// shown until the user dismisses it or the indicator's timer ends. Every method runs on the UI thread.
type Surface interface {
	ShowStatus(status Status)
	HideStatus()
	Present(result Result)
	Toast(message string)
}

// Options are the ports and host callbacks a Service runs on. Dispatch runs a function on the UI thread, Background
// runs one off it, and After schedules a function on the UI thread. Client returns the backend client of the account,
// or nil when it is signed out.
type Options struct {
	Clock        platform.Clock
	Clipboard    platform.Clipboard
	SelectedText platform.SelectedText
	Focus        platform.Focus
	Client       func() *backend.Client
	Connected    func() bool
	Surface      Surface
	Dispatch     func(fn func())
	Background   func(name string, fn func())
	After        func(d time.Duration, fn func())
}

// Service runs the triage command. It owns the state of the indicator, so only the UI thread touches it.
type Service struct {
	opts   Options
	reader *selection.Reader
	epoch  int
}

// NewService returns an idle service over opts.
func NewService(opts Options) *Service {
	return &Service{
		opts:   opts,
		reader: selection.NewReader(opts.Clock, opts.Clipboard, opts.SelectedText, opts.Focus),
	}
}

// Triage sends the text selected in source to the decision route with TriageQuestions and shows the answers.
func (s *Service) Triage(source platform.FocusTarget) {
	if !s.opts.Connected() {
		s.fail(TriageTitle, errNotSignedIn)
		return
	}
	s.show(Status{Title: TriageTitle, Subtitle: "Reading Selection", Tone: ToneLoading})
	s.opts.Background("ai-commands.triage.read", func() {
		text, err := s.reader.Read(source)
		s.opts.Dispatch(func() { s.triageRead(text, err) })
	})
}

// triageRead asks the decision route about the text a read returned.
func (s *Service) triageRead(text string, err error) {
	if err != nil {
		s.fail(TriageTitle, err)
		return
	}
	if strings.TrimSpace(text) == "" {
		s.fail(TriageTitle, errNoSelection)
		return
	}
	s.show(Status{Title: TriageTitle, Subtitle: "Deciding", Tone: ToneLoading})
	s.decide(TriageTitle, decisionFor(text), func(result backend.DecisionResult) {
		s.finish(Status{Title: TriageTitle, Subtitle: TriageSummary(result.Answers), Tone: ToneSuccess}, triageDismiss)
	}, triageTimeout)
}

// decide sends request off the UI thread and hands the answer to done on the UI thread. A failure is shown under title.
func (s *Service) decide(title string, request backend.DecisionRequest, done func(backend.DecisionResult), timeout time.Duration) {
	client := s.opts.Client()
	if client == nil {
		s.fail(title, errNotSignedIn)
		return
	}
	s.opts.Background("ai-commands.decide", func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, err := client.Decisions(ctx, request)
		s.opts.Dispatch(func() {
			if err != nil {
				s.fail(title, err)
				return
			}
			done(result)
		})
	})
}

// decisionFor is the request that asks TriageQuestions about text.
func decisionFor(text string) backend.DecisionRequest {
	return backend.DecisionRequest{Input: text, Questions: TriageQuestions}
}

// show sets the indicator to status and cancels any dismissal that was waiting.
func (s *Service) show(status Status) {
	s.epoch++
	s.opts.Surface.ShowStatus(status)
}

// finish shows status and closes it after d, unless another command showed something in the meantime.
func (s *Service) finish(status Status, d time.Duration) {
	s.show(status)
	epoch := s.epoch
	s.opts.After(d, func() {
		if epoch == s.epoch {
			s.hide()
		}
	})
}

// fail shows err's copy as an error status.
func (s *Service) fail(title string, err error) {
	s.finish(Status{Title: title, Subtitle: failureMessage(err), Tone: ToneError}, failureDismiss)
}

// hide closes the indicator and cancels any dismissal that was waiting.
func (s *Service) hide() {
	s.epoch++
	s.opts.Surface.HideStatus()
}

// Copy puts text on the clipboard as the user's own copy. A refused copy is said on the palette.
func (s *Service) Copy(text string) {
	if err := s.opts.Clipboard.WriteText(text); err != nil {
		s.opts.Surface.Toast("Copy failed.")
	}
}
