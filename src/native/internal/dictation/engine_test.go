package dictation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/dictation/audiomute"
	"github.com/pablopunk/nvm/native/internal/dictation/recordings"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// fakeAccount is a signed-in or signed-out account over one scripted transcriber.
type fakeAccount struct {
	signedIn    bool
	transcriber Transcriber
	notes       []error
}

func (a *fakeAccount) SignedIn() bool           { return a.signedIn }
func (a *fakeAccount) Transcriber() Transcriber { return a.transcriber }
func (a *fakeAccount) NoteError(err error)      { a.notes = append(a.notes, err) }

// memoryJournal keeps the output mute's journal in memory.
type memoryJournal struct{ entry *bool }

func (j *memoryJournal) Save(wasMuted bool) error {
	j.entry = &wasMuted
	return nil
}

func (j *memoryJournal) Load() (bool, bool, error) {
	if j.entry == nil {
		return false, false, nil
	}
	return *j.entry, true, nil
}

func (j *memoryJournal) Clear() error {
	j.entry = nil
	return nil
}

// engineRig is an engine over fakes, with the ports the tests read.
type engineRig struct {
	engine      *Engine
	mic         *fake.Microphone
	output      *fake.OutputMute
	transcriber *scriptedTranscriber
	files       *fake.FS
	store       *recordings.Store
}

func newEngineRig(t *testing.T, signedIn bool) engineRig {
	t.Helper()
	rig := engineRig{
		mic:         fake.NewMicrophone(),
		output:      fake.NewOutputMute(),
		transcriber: &scriptedTranscriber{},
		files:       fake.NewFS(),
	}
	if err := rig.files.MkdirAll("/data/dictation-recordings", 0o755); err != nil {
		t.Fatal(err)
	}
	rig.store = recordings.New(rig.files, "/data/dictation-recordings", func() time.Time { return time.UnixMilli(1_700_000_000_000) })
	rig.engine = NewEngine(EngineConfig{
		Microphone: rig.mic,
		Codec:      fake.NewOpusCodec(),
		Account:    &fakeAccount{signedIn: signedIn, transcriber: rig.transcriber},
		Recordings: rig.store,
		Muter:      audiomute.New(rig.output, &memoryJournal{}),
		Background: func(_ string, fn func()) { fn() },
	})
	return rig
}

func TestEngineStatusFollowsTheDictationPhase(t *testing.T) {
	rig := newEngineRig(t, true)
	if got := rig.engine.Status(); got != api.DictationIdle {
		t.Fatalf("status %q; want idle", got)
	}
	if err := rig.engine.Start(api.DictationStartOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := rig.engine.Status(); got != api.DictationRecording {
		t.Fatalf("status %q; want recording", got)
	}
	if err := rig.engine.Cancel(); err != nil {
		t.Fatal(err)
	}
	if got := rig.engine.Status(); got != api.DictationIdle {
		t.Fatalf("status %q after cancel; want idle", got)
	}
}

func TestEngineAPIAvailableIsTheSignedInAccount(t *testing.T) {
	if !newEngineRig(t, true).engine.APIAvailable() {
		t.Fatal("a signed-in account is not available")
	}
	if newEngineRig(t, false).engine.APIAvailable() {
		t.Fatal("a signed-out account is available")
	}
}

func TestEngineDevicesListTheMicrophonesWithTheDefaultMarked(t *testing.T) {
	rig := newEngineRig(t, true)
	rig.mic.SetDevices(platform.MicrophoneDevice{ID: "usb", Title: "USB Mic"}, platform.MicrophoneDevice{ID: "built-in", Title: "Built-in", Default: true})
	devices, err := rig.engine.Devices()
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].ID != "usb" || devices[0].IsDefault || devices[1].Title != "Built-in" || !devices[1].IsDefault {
		t.Fatalf("devices %+v; want both microphones, the built-in one default", devices)
	}
}

func TestEngineDevicesRefuseWhenTheListCannotBeRead(t *testing.T) {
	rig := newEngineRig(t, true)
	rig.mic.FailNext("microphone.devices", platform.ErrDenied)
	_, err := rig.engine.Devices()
	if !errors.Is(err, api.ErrPermission) {
		t.Fatalf("err %v; want the permission cause", err)
	}
}

func TestEngineStartOpensTheDefaultMicrophoneAndMutesTheOutput(t *testing.T) {
	rig := newEngineRig(t, true)
	if err := rig.engine.Start(api.DictationStartOptions{DeviceID: "default", MuteSystemAudioWhileRecord: true}); err != nil {
		t.Fatal(err)
	}
	if opened := rig.mic.Opened(); len(opened) != 1 || opened[0] != "" {
		t.Fatalf("opened %v; want the system default", opened)
	}
	if muted, _ := rig.output.Muted(); !muted {
		t.Fatal("the output is not muted while recording")
	}
	if _, err := rig.engine.Stop(); err != nil {
		t.Fatal(err)
	}
	if muted, _ := rig.output.Muted(); muted {
		t.Fatal("the output stayed muted after the dictation stopped")
	}
}

func TestEngineStartFailureGivesTheOutputBackAndNamesTheCause(t *testing.T) {
	rig := newEngineRig(t, true)
	rig.mic.FailNext("microphone.open", platform.ErrDenied)
	err := rig.engine.Start(api.DictationStartOptions{MuteSystemAudioWhileRecord: true})
	if !errors.Is(err, api.ErrPermission) || err.Error() == "" {
		t.Fatalf("err %v; want a permission failure with the user's words", err)
	}
	if muted, _ := rig.output.Muted(); muted {
		t.Fatal("a failed start left the output muted")
	}
}

func TestEngineStopReturnsTheHostedTranscript(t *testing.T) {
	rig := newEngineRig(t, true)
	rig.transcriber.texts = []string{"hello world"}
	rig.mic.Queue(speech(0.5, 9000)...)
	if err := rig.engine.Start(api.DictationStartOptions{}); err != nil {
		t.Fatal(err)
	}
	text, err := rig.engine.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello world" {
		t.Fatalf("transcript %q", text)
	}
}

func TestEngineStopWithoutADictationIsAFailure(t *testing.T) {
	if _, err := newEngineRig(t, true).engine.Stop(); err == nil || !errors.Is(err, api.ErrNotAvailable) {
		t.Fatalf("err %v; want an unavailable failure", err)
	}
}

func TestEngineCancelEndsTheDictationAndStopThenFails(t *testing.T) {
	rig := newEngineRig(t, true)
	if err := rig.engine.Start(api.DictationStartOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := rig.engine.Cancel(); err != nil {
		t.Fatal(err)
	}
	if got := rig.engine.Status(); got != api.DictationIdle {
		t.Fatalf("status %q after cancel; want idle", got)
	}
	if _, err := rig.engine.Stop(); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatalf("stop after cancel err %v; want the unavailable cause", err)
	}
}

func TestEngineRecordingsListAndDeleteTheSavedAudio(t *testing.T) {
	rig := newEngineRig(t, true)
	if err := rig.store.Save("dictation-one", [][]byte{[]byte("segment")}); err != nil {
		t.Fatal(err)
	}
	saved, err := rig.engine.Recordings()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].ID != "dictation-one" || saved[0].SegmentCount != 1 || saved[0].CreatedAtMs != 1_700_000_000_000 {
		t.Fatalf("recordings %+v; want the saved dictation", saved)
	}
	if err := rig.engine.DeleteRecording("dictation-one"); err != nil {
		t.Fatal(err)
	}
	if left, _ := rig.engine.Recordings(); len(left) != 0 {
		t.Fatalf("recordings %+v after delete; want none", left)
	}
}

func TestEngineRetryTranscribesASavedDictationAgain(t *testing.T) {
	rig := newEngineRig(t, true)
	if err := rig.store.Save("dictation-retry", [][]byte{[]byte("webm bytes")}); err != nil {
		t.Fatal(err)
	}
	rig.transcriber.texts = []string{"second try"}
	text, err := rig.engine.Retry(context.Background(), "dictation-retry")
	if err != nil {
		t.Fatal(err)
	}
	if text != "second try" {
		t.Fatalf("text %q; want the retried transcript", text)
	}
}

func TestEngineRecordingsWithoutAStoreAreEmpty(t *testing.T) {
	engine := NewEngine(EngineConfig{Microphone: fake.NewMicrophone(), Codec: fake.NewOpusCodec(), Account: &fakeAccount{}})
	if saved, err := engine.Recordings(); err != nil || len(saved) != 0 {
		t.Fatalf("recordings %+v, %v; want none", saved, err)
	}
	if err := engine.DeleteRecording("x"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("delete err %v; want not found", err)
	}
}

func TestEngineMicrophoneIDMapsTheDefaultSpellings(t *testing.T) {
	if microphoneID("default") != "" || microphoneID("") != "" || microphoneID("usb") != "usb" {
		t.Fatal("the default spellings must name the system microphone")
	}
}

func TestEngineDeliversCapturedMicrophoneLevels(t *testing.T) {
	mic := fake.NewMicrophone()
	mic.Queue(speech(0.2, 8192)...)
	levels := make(chan float64, 4)
	engine := NewEngine(EngineConfig{
		Microphone: mic,
		Codec:      fake.NewOpusCodec(),
		OnLevel:    func(level float64, _ func() bool) { levels <- level },
	})
	if err := engine.Start(api.DictationStartOptions{}); err != nil {
		t.Fatal(err)
	}
	select {
	case level := <-levels:
		if level != 0.5 {
			t.Fatalf("level = %v, want 0.5", level)
		}
	case <-time.After(time.Second):
		t.Fatal("no microphone level reached the engine callback")
	}
	_ = engine.Cancel()
}
