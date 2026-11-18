package dictation

import (
	"errors"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type gatedDictation struct {
	api.Dictation
	entered       chan struct{}
	release       chan struct{}
	deviceRelease chan struct{}
	err           error
}

func (d *gatedDictation) APIAvailable() (bool, error) { return true, nil }
func (d *gatedDictation) Start(api.DictationStartOptions) error {
	close(d.entered)
	<-d.release
	return d.err
}
func (d *gatedDictation) Devices() ([]api.DictationDevice, error) {
	<-d.deviceRelease
	return []api.DictationDevice{{ID: "default", Title: "Mic", IsDefault: true}}, nil
}

type lockedIndicators struct {
	api.Indicators
	mu    sync.Mutex
	shown []api.IndicatorInput
}

func (i *lockedIndicators) Show(in api.IndicatorInput) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.shown = append(i.shown, in)
	return nil
}
func (i *lockedIndicators) Hide(string) error { return nil }
func (i *lockedIndicators) snapshot() []api.IndicatorInput {
	i.mu.Lock()
	defer i.mu.Unlock()
	return append([]api.IndicatorInput(nil), i.shown...)
}

type gatedContext struct {
	fakeContext
	recorder *gatedDictation
	surface  *lockedIndicators
}

func (c gatedContext) Dictation() api.Dictation   { return c.recorder }
func (c gatedContext) Indicators() api.Indicators { return c.surface }
func TestIndicatorWaitsForHostReadinessAndIgnoresLateName(t *testing.T) {
	for _, failure := range []error{nil, errors.New("failed"), errors.New("")} {
		d := &gatedDictation{entered: make(chan struct{}), release: make(chan struct{}), deviceRelease: make(chan struct{}), err: failure}
		surface := &lockedIndicators{}
		ctx := gatedContext{fakeContext: newContext(), recorder: d, surface: surface}
		done := make(chan struct{})
		go func() { _, _ = New(Legacy{}).start(ctx); close(done) }()
		<-d.entered
		history := surface.snapshot()
		if len(history) != 1 || history[0].Status != api.IndicatorLoading {
			t.Fatalf("before readiness: %+v", history)
		}
		close(d.release)
		<-done
		history = surface.snapshot()
		for _, item := range history[:len(history)-1] {
			if item.Status == api.IndicatorRecording {
				t.Fatalf("premature recording: %+v", history)
			}
		}
		if failure == nil && history[len(history)-1].Status != api.IndicatorRecording {
			t.Fatalf("after readiness: %+v", history)
		}
		if failure != nil && history[len(history)-1].Status == api.IndicatorRecording {
			t.Fatalf("after failure: %+v", history)
		}
		close(d.deviceRelease)
	}
}
func TestLateIndicatorRefinementCannotReplaceFinishedState(t *testing.T) {
	surface := &lockedIndicators{}
	old := newDeferredIndicator(surface)
	old.finish()
	_ = surface.Show(transcribingIndicator())
	old.refine(waitingIndicator("late"))
	history := surface.snapshot()
	if history[len(history)-1].Subtitle != "Transcribing" {
		t.Fatalf("late metadata replaced current state: %+v", history)
	}
}

func TestDeviceNameRefinesOnlyThePendingIndicator(t *testing.T) {
	surface := &lockedIndicators{}
	waiting := newDeferredIndicator(surface)
	waiting.refine(waitingIndicator("USB Mic"))
	waiting.finish()
	waiting.refine(waitingIndicator("Old Mic"))
	history := surface.snapshot()
	if len(history) != 2 || history[0].Subtitle != "Waiting for USB Mic" || history[0].Status != api.IndicatorLoading || history[1].Status != api.IndicatorRecording {
		t.Fatalf("indicator history = %+v", history)
	}
}
