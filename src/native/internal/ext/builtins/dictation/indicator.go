package dictation

import (
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// IndicatorID identifies the dictation indicator.
const IndicatorID = "dictation"

// Terminal indicators stay on screen long enough to read.
const (
	noSpeechDuration = 2200 * time.Millisecond
	errorDuration    = 4000 * time.Millisecond
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

// deferredIndicator serializes preparation updates and rejects late device metadata.
type deferredIndicator struct {
	indicators api.Indicators
	mu         sync.Mutex
	pending    bool
}

func newDeferredIndicator(indicators api.Indicators) *deferredIndicator {
	return &deferredIndicator{indicators: indicators, pending: true}
}
func (d *deferredIndicator) refine(input api.IndicatorInput) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pending {
		_ = d.indicators.Show(input)
	}
}
func (d *deferredIndicator) finish() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pending {
		d.pending = false
		_ = d.indicators.Show(listeningIndicator())
	}
}
func (d *deferredIndicator) cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pending = false
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
