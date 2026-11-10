package dictation

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/cleanup"
)

// toggle starts a dictation when none runs and stops the running one. A key pressed while the microphone is still
// opening, or while the transcript is on its way, does nothing, as 1.x did.
func (e Extension) toggle(ctx api.Context) (api.Result, error) {
	if e.session.isStarting() {
		return api.Result{}, nil
	}
	status, err := ctx.Dictation().Status()
	if err != nil {
		return api.Result{Toast: &api.Toast{Message: err.Error(), Tone: "error"}}, nil
	}
	switch status {
	case api.DictationTranscribing:
		return api.Result{}, nil
	case api.DictationIdle:
		return e.start(ctx)
	}
	return e.stop(ctx)
}

// start opens the microphone with the chosen device, muting the output while it records. The indicator says so while
// the microphone opens, and names the device once its title is known.
func (e Extension) start(ctx api.Context) (api.Result, error) {
	settings := e.readSettings(ctx)
	cleanWithAI := settings.CleanupWithAI && aiAvailable(ctx)
	e.session.beginStart(frontmostAppID(ctx))
	defer e.session.endStart()
	showIndicator(ctx, listeningIndicator())
	if cleanWithAI {
		prepareCleanup(ctx)
	}
	if available, err := ctx.Dictation().APIAvailable(); err != nil || !available {
		showIndicator(ctx, failureIndicator("Sign in to use cloud dictation"))
		return api.Result{}, nil
	}
	deferred := newDeferredIndicator(ctx.Indicators())
	deferred.begin(waitingIndicator(""))
	go nameMicrophone(ctx, settings.DeviceID, deferred)
	err := ctx.Dictation().Start(api.DictationStartOptions{IndicatorID: IndicatorID, DeviceID: settings.DeviceID, MuteSystemAudioWhileRecord: true})
	if err != nil {
		deferred.cancel()
		showFailure(ctx, err)
		return api.Result{}, nil
	}
	deferred.finish()
	return api.Result{}, nil
}

// showFailure shows why a dictation failed. A failure with no words is a cancellation, which closes the indicator.
func showFailure(ctx api.Context, err error) {
	if message := err.Error(); message != "" {
		showIndicator(ctx, failureIndicator(message))
		return
	}
	hideIndicator(ctx)
}

// stop ends the recording, cleans the transcript when the settings ask, saves it to the history and pastes it into the
// app that had focus when the dictation started. The screen words are read while the transcript is on its way.
func (e Extension) stop(ctx api.Context) (api.Result, error) {
	settings := e.readSettings(ctx)
	cleanWithAI := settings.CleanupWithAI && aiAvailable(ctx)
	target := e.session.takeTarget()
	showIndicator(ctx, transcribingIndicator())
	screen := screenTermsAsync(ctx, settings, cleanWithAI)
	transcript, err := ctx.Dictation().Stop()
	if err != nil {
		showFailure(ctx, err)
		return api.Result{}, nil
	}
	if strings.TrimSpace(transcript) == "" {
		showIndicator(ctx, noSpeechIndicator())
		return api.Result{}, nil
	}
	if cleanWithAI {
		showIndicator(ctx, cleaningIndicator())
	}
	text := cleanTranscript(ctx, transcript, cleanWithAI, settings.Dictionary, <-screen)
	e.saveTranscript(ctx, text)
	hideIndicator(ctx)
	return api.Result{Next: pasteAction(text, settings, target)}, nil
}

// pasteAction pastes the transcript into the app the dictation started in, concealed from clipboard history. Unless the
// settings keep the text on the clipboard, the previous clipboard comes back once the paste has read it.
func pasteAction(text string, settings Settings, target string) *api.Action {
	paste := api.PasteText(pasteTitle, text)
	paste.DismissAfterRun = true
	keep := settings.CopyToClipboard
	paste.Paste = &api.PasteOptions{
		Concealed:           !keep,
		RestoreClipboard:    !keep,
		ExpectedFrontmostID: target,
	}
	return &paste
}

// frontmostAppID names the app that has the keyboard now. A name the OS cannot give is "", and the paste goes to whatever
// app is in front when it is sent.
func frontmostAppID(ctx api.Context) string {
	app, err := ctx.Desktop().Apps().Frontmost()
	if err != nil {
		return ""
	}
	return app.ID
}

// nameMicrophone shows the device a waiting dictation waits for, once the device list is read.
func nameMicrophone(ctx api.Context, deviceID string, deferred *deferredIndicator) {
	devices, err := ctx.Dictation().Devices()
	if err != nil {
		return
	}
	if title := deviceTitle(devices, deviceID); title != "" {
		deferred.refine(waitingIndicator(title))
	}
}

// deviceTitle is the title of the microphone deviceID names. "default" and "" name the system default microphone.
func deviceTitle(devices []api.DictationDevice, deviceID string) string {
	for _, device := range devices {
		if device.ID == deviceID || (isDefaultDevice(deviceID) && device.IsDefault) {
			return device.Title
		}
	}
	return ""
}

func isDefaultDevice(deviceID string) bool {
	return deviceID == "" || deviceID == defaultDeviceID
}

// prepareCleanup warms the fast model while the user speaks, so the cleanup at the end waits less.
func prepareCleanup(ctx api.Context) {
	go func() {
		_ = ctx.AI().Prepare(api.AIOptions{Model: api.ModelFast, System: cleanup.System})
	}()
}

// screenTermsAsync reads the words on screen in the background when the cleanup will use them. The result is always
// sent, empty when no words are read.
func screenTermsAsync(ctx api.Context, settings Settings, cleanWithAI bool) <-chan []string {
	terms := make(chan []string, 1)
	if !(cleanWithAI && settings.UseScreenContext) || !ctx.System().HasCapability("ocr") {
		terms <- nil
		return terms
	}
	go func() { terms <- readScreenTerms(ctx) }()
	return terms
}

func (s *session) isStarting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starting
}

func (s *session) beginStart(target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starting, s.target = true, target
}

func (s *session) endStart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starting = false
}

// takeTarget returns the app the dictation started in, and forgets it.
func (s *session) takeTarget() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := s.target
	s.target = ""
	return target
}
