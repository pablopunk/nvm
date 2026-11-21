package supervisor

import (
	"context"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/services"
)

// fakeDictationEngine answers the dictation calls of the fixture from a script and records what it was asked.
type fakeDictationEngine struct {
	status     api.DictationStatus
	transcript string
	started    []api.DictationStartOptions
	cancelled  bool
	deleted    []string
	retried    []string
}

func (f *fakeDictationEngine) Status() api.DictationStatus { return f.status }
func (f *fakeDictationEngine) APIAvailable() bool          { return true }
func (f *fakeDictationEngine) Devices() ([]api.DictationDevice, error) {
	return []api.DictationDevice{{ID: "usb", Title: "USB Mic", IsDefault: true}}, nil
}

func (f *fakeDictationEngine) Start(options api.DictationStartOptions) error {
	f.started = append(f.started, options)
	f.status = api.DictationRecording
	return nil
}

func (f *fakeDictationEngine) Stop() (string, error) {
	f.status = api.DictationIdle
	return f.transcript, nil
}

func (f *fakeDictationEngine) Cancel() error {
	f.cancelled = true
	return nil
}

func (f *fakeDictationEngine) Recordings() ([]api.SavedDictation, error) {
	return []api.SavedDictation{{ID: "dictation-saved", CreatedAtMs: 1, SegmentCount: 3}}, nil
}

func (f *fakeDictationEngine) Retry(_ context.Context, id string) (string, error) {
	f.retried = append(f.retried, id)
	return "retried text", nil
}

func (f *fakeDictationEngine) DeleteRecording(id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

var _ services.DictationEngine = (*fakeDictationEngine)(nil)

// TestDictationFixtureRunsThroughTheChild runs the dictation fixture in a child process: every ctx.Dictation call
// reaches the host engine through the protocol and its answer comes back to the extension.
func TestDictationFixtureRunsThroughTheChild(t *testing.T) {
	engine := &fakeDictationEngine{status: api.DictationIdle, transcript: "hello from the mic"}
	rig := newServicesRig(t)
	rig.deps.Dictation = engine
	runtime := servicedRuntime(t, "acme.dictation", rig, fixtureManifest("acme.dictation", api.CapabilityDictation))

	if got := rowTitle(t, runtime, "status"); got != "status: idle" {
		t.Fatalf("status row %q", got)
	}
	if got := rowTitle(t, runtime, "available"); got != "available: true" {
		t.Fatalf("available row %q", got)
	}
	if got := rowTitle(t, runtime, "devices"); got != "devices: USB Mic" {
		t.Fatalf("devices row %q", got)
	}
	if got := rowTitle(t, runtime, "dictate"); got != "transcript: hello from the mic" {
		t.Fatalf("dictate row %q", got)
	}
	if len(engine.started) != 1 || engine.started[0] != (api.DictationStartOptions{DeviceID: "default", MuteSystemAudioWhileRecord: true}) {
		t.Fatalf("started %+v; want the default microphone with the output muted", engine.started)
	}
	if got := rowTitle(t, runtime, "cancel"); got != "cancel: done" || !engine.cancelled {
		t.Fatalf("cancel row %q, cancelled %t", got, engine.cancelled)
	}
	if got := rowTitle(t, runtime, "recordings"); got != "recordings: dictation-saved/3" {
		t.Fatalf("recordings row %q", got)
	}
	if got := rowTitle(t, runtime, "retry"); got != "retry: retried text" || len(engine.retried) != 1 {
		t.Fatalf("retry row %q, retried %v", got, engine.retried)
	}
	if got := rowTitle(t, runtime, "delete"); got != "delete: done" || len(engine.deleted) != 1 {
		t.Fatalf("delete row %q, deleted %v", got, engine.deleted)
	}
}

// TestDictationRefusedWithoutTheCapabilityInTheChild shows an extension that did not declare dictation is refused.
func TestDictationRefusedWithoutTheCapabilityInTheChild(t *testing.T) {
	engine := &fakeDictationEngine{}
	rig := newServicesRig(t)
	rig.deps.Dictation = engine
	runtime := servicedRuntime(t, "acme.dictation", rig, fixtureManifest("acme.dictation"))
	if got := rowTitle(t, runtime, "dictate"); !strings.Contains(got, "did not declare the dictation capability") {
		t.Fatalf("row %q; want the undeclared-capability refusal", got)
	}
	if len(engine.started) != 0 {
		t.Fatal("a refused dictation reached the engine")
	}
}
