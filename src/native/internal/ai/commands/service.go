package commands

import (
	"context"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Titles of the commands, as the palette and the indicator name them.
const (
	FixTitle      = "Fix Selected Text with AI"
	TriageTitle   = "Triage Selected Text with AI"
	ClassifyTitle = "Classify Clipboard Image with AI"
)

// Dismissal delays: a result the user should read stays longer than a failure's one-line copy would need.
const (
	successDismiss = 2 * time.Second
	triageDismiss  = 6 * time.Second
	failureDismiss = 4 * time.Second
	triageTimeout  = 30 * time.Second
	classifyTimeout = 30 * time.Second
	restoreDelay   = 500 * time.Millisecond
)

// Surface is what the commands draw on. The indicator is shown while a command works, and a result or a failure is
// shown until the user dismisses it or the indicator's timer ends. Every method runs on the UI thread.
type Surface interface {
	ShowStatus(status Status)
	HideStatus()
	Present(result Result)
	Toast(message string)
	// DismissPalette hides the palette and drops the presentation's pending keys, so a paste that waits for the hide
	// waits for this one.
	DismissPalette()
}

// Options are the ports and host callbacks a Service runs on. Dispatch runs a function on the UI thread, Background
// runs one off it, and After schedules a function on the UI thread. Chats is the one chat service of the app; Client
// returns the backend client of the account, or nil when it is signed out.
type Options struct {
	Clock         platform.Clock
	Clipboard     platform.Clipboard
	SelectedText  platform.SelectedText
	Paster        platform.Paster
	Focus         platform.Focus
	Chats         *chat.Service
	Client        func() *backend.Client
	Connected     func() bool
	Surface       Surface
	Dispatch      func(fn func())
	Background    func(name string, fn func())
	After         func(d time.Duration, fn func())
	RestoreDelay  time.Duration
}

// Service runs the AI commands. It owns the state of the indicator and of the proofread attempts, so only the UI
// thread touches it.
type Service struct {
	opts     Options
	reader   *Reader
	attempts map[string]attempt
	epoch    int
}

// attempt is one proofread request in flight: the chat that carries it, the text it corrects and where the
// correction goes.
type attempt struct {
	input  string
	retry  bool
	source platform.FocusTarget
}

// NewService returns an idle service over opts.
func NewService(opts Options) *Service {
	if opts.RestoreDelay == 0 {
		opts.RestoreDelay = restoreDelay
	}
	return &Service{
		opts:     opts,
		reader:   NewReader(opts.Clock, opts.Clipboard, opts.SelectedText, opts.Focus),
		attempts: map[string]attempt{},
	}
}

// Fix proofreads the text selected in source. The reply streams into a proofread chat; when it is a valid correction
// the result is shown, and Replace puts it back into source.
func (s *Service) Fix(source platform.FocusTarget) {
	if !s.opts.Connected() {
		s.fail(FixTitle, errNotSignedIn)
		return
	}
	s.show(Status{Title: FixTitle, Subtitle: "Reading Selection", Tone: ToneLoading})
	s.opts.Background("ai-commands.fix.read", func() {
		text, err := s.reader.Read(source)
		s.opts.Dispatch(func() { s.fixRead(text, err, source) })
	})
}

// fixRead starts the proofread of the text a read returned.
func (s *Service) fixRead(text string, err error, source platform.FocusTarget) {
	switch {
	case err != nil:
		s.fail(FixTitle, err)
	case strings.TrimSpace(text) == "":
		s.fail(FixTitle, errNoSelection)
	default:
		s.startProofread(attempt{input: text, source: source})
	}
}

// startProofread sends one proofread to a new chat of the proofread kind.
func (s *Service) startProofread(a attempt) {
	subtitle := "Fixing Text"
	if a.retry {
		subtitle = "Retrying Correction"
	}
	s.show(Status{Title: FixTitle, Subtitle: subtitle, Tone: ToneLoading})
	record, err := s.opts.Chats.NewChat(chat.KindProofread, chat.TierFast)
	if err != nil {
		s.fail(FixTitle, err)
		return
	}
	if err := s.opts.Chats.Send(record.ID, ProofreadPrompt(a.input, a.retry)); err != nil {
		s.fail(FixTitle, err)
		return
	}
	s.attempts[record.ID] = a
}

// Settled takes the end of a proofread chat's turn from the chat service. Chats that are not proofreads are ignored.
// A valid correction is presented; an invalid one is retried once, then reported.
func (s *Service) Settled(chatID string, outcome chat.Outcome) {
	a, ok := s.attempts[chatID]
	if !ok {
		return
	}
	delete(s.attempts, chatID)
	switch outcome.Ending {
	case chat.EndFailed:
		s.fail(FixTitle, outcome.Err)
		return
	case chat.EndStopped:
		s.hide()
		return
	}
	reply := replyOf(s.opts.Chats, chatID)
	s.judgeProofread(a, reply)
}

// judgeProofread decides what a finished proofread reply means for the selection.
func (s *Service) judgeProofread(a attempt, reply string) {
	switch {
	case strings.TrimSpace(reply) == "":
		s.fail(FixTitle, errNoCorrection)
	case !PreservesInput(a.input, reply):
		if a.retry {
			s.fail(FixTitle, errInvalidCorrection)
			return
		}
		s.startProofread(attempt{input: a.input, retry: true, source: a.source})
	case reply == a.input:
		s.finish(Status{Title: FixTitle, Subtitle: "No changes needed", Tone: ToneSuccess}, successDismiss)
	default:
		s.hide()
		s.opts.Surface.Present(Result{
			Title:   "Fixed Text",
			Text:    func() string { return reply },
			Replace: func(text string) { s.Replace(text, a.source) },
			Copy:    s.Copy,
		})
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

// ClassifyClipboardImage classifies the image on the clipboard and presents the category, which Copy takes.
func (s *Service) ClassifyClipboardImage() {
	if !s.opts.Connected() {
		s.fail(ClassifyTitle, errNotSignedIn)
		return
	}
	s.show(Status{Title: ClassifyTitle, Subtitle: "Reading Image", Tone: ToneLoading})
	s.opts.Background("ai-commands.classify.read", func() {
		png, err := s.opts.Clipboard.ReadImagePNG()
		s.opts.Dispatch(func() { s.classifyRead(png, err) })
	})
}

// classifyRead sends the image a clipboard read returned to the decision route.
func (s *Service) classifyRead(png []byte, err error) {
	switch {
	case err != nil:
		s.fail(ClassifyTitle, errNoImage)
		return
	case len(png) > MaximumImageBytes:
		s.fail(ClassifyTitle, errImageTooLarge)
		return
	}
	s.show(Status{Title: ClassifyTitle, Subtitle: "Classifying", Tone: ToneLoading})
	request := backend.DecisionRequest{Input: classifyInput, Images: []backend.DecisionImage{pngImage(png)}, Questions: ClassifyQuestions}
	s.decide(ClassifyTitle, request, func(result backend.DecisionResult) {
		s.presentCategory(result)
	}, classifyTimeout)
}

// presentCategory shows the category an image classification answered with, or fails when there is none.
func (s *Service) presentCategory(result backend.DecisionResult) {
	category, ok := result.Answers["category"]
	if !ok || category.Type != "choice" {
		s.fail(ClassifyTitle, errNoCategory)
		return
	}
	s.hide()
	s.opts.Surface.Present(Result{Title: "Image Classification", Text: func() string { return category.Choice }, Copy: s.Copy})
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

// replyOf returns the text of the chat's last reply, or "" when the chat is gone.
func replyOf(chats *chat.Service, chatID string) string {
	record, ok := chats.Chat(chatID)
	if !ok {
		return ""
	}
	text, _ := chat.LastReply(record)
	return text
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
