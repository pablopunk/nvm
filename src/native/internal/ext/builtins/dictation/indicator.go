package dictation

import (
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// IndicatorID identifies the dictation indicator.
const IndicatorID = "dictation"

// Indicator timings: a microphone that takes longer than this to open shows that it is waiting, and the terminal states
// stay on screen long enough to read.
const (
	waitingIndicatorDelay = time.Second
	noSpeechDuration      = 2200 * time.Millisecond
	errorDuration         = 4000 * time.Millisecond
)

// indicatorInput is the dictation indicator in one state.
func indicatorInput(subtitle string, status api.IndicatorStatus) api.IndicatorInput {
	return api.IndicatorInput{ID: IndicatorID, Title: dictateTitle, Subtitle: subtitle, Status: status}
}

// The states of the dictation indicator, in the order a dictation passes through them.
func listeningIndicator() api.IndicatorInput {
	return indicatorInput("Listening", api.IndicatorRecording)
}

func waitingIndicator(device string) api.IndicatorInput {
	subtitle := "Waiting for microphone"
	if device != "" {
		subtitle = "Waiting for " + device
	}
	return indicatorInput(subtitle, api.IndicatorLoading)
}

func transcribingIndicator() api.IndicatorInput {
	return indicatorInput("Transcribing", api.IndicatorTranscribing)
}

func cleaningIndicator() api.IndicatorInput {
	return indicatorInput("Cleaning", api.IndicatorLoading)
}

func noSpeechIndicator() api.IndicatorInput {
	input := indicatorInput("No speech detected", "")
	input.DurationMs = noSpeechDuration.Milliseconds()
	return input
}

// failureIndicator shows a failed dictation's words, which the host also keeps in the log.
func failureIndicator(message string) api.IndicatorInput {
	input := indicatorInput("Dictation unavailable: "+message, api.IndicatorError)
	input.DurationMs = errorDuration.Milliseconds()
	return input
}

// deferredIndicator shows the waiting state only when the microphone takes longer than waitingIndicatorDelay to open. A
// start that opens in time never flickers the waiting state, and a finished start shows listening again.
type deferredIndicator struct {
	indicators api.Indicators
	mu         sync.Mutex
	timer      *time.Timer
	pending    api.IndicatorInput
	hasPending bool
	visible    bool
}

// newDeferredIndicator returns a deferred indicator that draws on indicators.
func newDeferredIndicator(indicators api.Indicators) *deferredIndicator {
	return &deferredIndicator{indicators: indicators}
}

// begin schedules the waiting state, and cancels any state scheduled before it.
func (d *deferredIndicator) begin(input api.IndicatorInput) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopTimerLocked()
	if d.visible {
		d.show(listeningIndicator())
	}
	d.visible = false
	d.pending, d.hasPending = input, true
	d.timer = time.AfterFunc(waitingIndicatorDelay, d.reveal)
}

// reveal shows the scheduled state once its delay has passed.
func (d *deferredIndicator) reveal() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.timer = nil
	if !d.hasPending {
		return
	}
	d.visible = true
	d.show(d.pending)
}

// refine replaces the scheduled state, and the shown one when it is already visible.
func (d *deferredIndicator) refine(input api.IndicatorInput) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.hasPending {
		return
	}
	d.pending = input
	if d.visible {
		d.show(input)
	}
}

// finish ends the waiting: a visible waiting state turns into listening, and nothing is scheduled any more.
func (d *deferredIndicator) finish() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopTimerLocked()
	d.hasPending = false
	if d.visible {
		d.show(listeningIndicator())
	}
	d.visible = false
}

// cancel drops the scheduled state without showing anything.
func (d *deferredIndicator) cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopTimerLocked()
	d.hasPending, d.visible = false, false
}

func (d *deferredIndicator) stopTimerLocked() {
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

func (d *deferredIndicator) show(input api.IndicatorInput) {
	_ = d.indicators.Show(input)
}

// showIndicator draws input through the extension's indicator surface. A surface that refuses is not a dictation
// failure, so the refusal is dropped.
func showIndicator(ctx api.Context, input api.IndicatorInput) {
	_ = ctx.Indicators().Show(input)
}

// hideIndicator closes the dictation indicator.
func hideIndicator(ctx api.Context) {
	_ = ctx.Indicators().Hide(IndicatorID)
}
