package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// fakeDictationEngine records each call and answers from its script.
type fakeDictationEngine struct {
	status     api.DictationStatus
	available  bool
	devices    []api.DictationDevice
	started    []api.DictationStartOptions
	startErr   error
	transcript string
	stopErr    error
	cancelled  int
	recordings []api.SavedDictation
	retried    []string
	retryErr   error
	deleted    []string
}

func (f *fakeDictationEngine) Status() api.DictationStatus { return f.status }
func (f *fakeDictationEngine) APIAvailable() bool          { return f.available }
func (f *fakeDictationEngine) Devices() ([]api.DictationDevice, error) {
	return f.devices, nil
}

func (f *fakeDictationEngine) Start(options api.DictationStartOptions) error {
	f.started = append(f.started, options)
	return f.startErr
}

func (f *fakeDictationEngine) Stop() (string, error) { return f.transcript, f.stopErr }

func (f *fakeDictationEngine) Cancel() error {
	f.cancelled++
	return nil
}

func (f *fakeDictationEngine) Recordings() ([]api.SavedDictation, error) {
	return f.recordings, nil
}

func (f *fakeDictationEngine) Retry(_ context.Context, id string) (string, error) {
	f.retried = append(f.retried, id)
	return f.transcript, f.retryErr
}

func (f *fakeDictationEngine) DeleteRecording(id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

// dictationHandler returns the services of an extension over engine, declaring the given capabilities.
func dictationHandler(t *testing.T, engine DictationEngine, capabilities ...api.Capability) *Handler {
	t.Helper()
	handler := New(Deps{Clock: fake.NewClock(), DataRoot: t.TempDir(), Dictation: engine},
		manifestFor("acme.dictation", capabilities...))
	t.Cleanup(func() { _ = handler.Close() })
	return handler
}

// dictationCall answers one dictation operation of handler, as the child's call would be answered.
func dictationCall(t *testing.T, handler *Handler, op string, args any) protocol.ServiceReply {
	t.Helper()
	var raw json.RawMessage
	if args != nil {
		encoded, err := json.Marshal(args)
		if err != nil {
			t.Fatalf("arguments: %v", err)
		}
		raw = encoded
	}
	return handler.Serve(context.Background(), protocol.ServiceCall{Service: protocol.ServiceDictation, Op: op, Args: raw})
}

func TestDictationStatusAndAvailabilityReachTheEngine(t *testing.T) {
	engine := &fakeDictationEngine{status: api.DictationTranscribing, available: true}
	handler := dictationHandler(t, engine, api.CapabilityDictation)
	var status api.DictationStatus
	decodeReply(t, dictationCall(t, handler, protocol.OpStatus, nil), &status)
	if status != api.DictationTranscribing {
		t.Fatalf("status %q", status)
	}
	var available bool
	decodeReply(t, dictationCall(t, handler, protocol.OpAPIAvailable, nil), &available)
	if !available {
		t.Fatal("available = false; want true")
	}
}

func TestDictationDevicesAreListed(t *testing.T) {
	engine := &fakeDictationEngine{devices: []api.DictationDevice{{ID: "usb", Title: "USB Mic", IsDefault: true}}}
	handler := dictationHandler(t, engine, api.CapabilityDictation)
	var devices []api.DictationDevice
	decodeReply(t, dictationCall(t, handler, protocol.OpDevices, nil), &devices)
	if len(devices) != 1 || devices[0].Title != "USB Mic" || !devices[0].IsDefault {
		t.Fatalf("devices %+v", devices)
	}
}

func TestDictationStartPassesTheOptionsThrough(t *testing.T) {
	engine := &fakeDictationEngine{}
	handler := dictationHandler(t, engine, api.CapabilityDictation)
	reply := dictationCall(t, handler, protocol.OpStart, api.DictationStartOptions{DeviceID: "usb", MuteSystemAudioWhileRecord: true})
	if reply.Failure != nil {
		t.Fatalf("start failed: %+v", *reply.Failure)
	}
	if len(engine.started) != 1 || engine.started[0] != (api.DictationStartOptions{DeviceID: "usb", MuteSystemAudioWhileRecord: true}) {
		t.Fatalf("started %+v", engine.started)
	}
}

func TestDictationStopReturnsTheTranscript(t *testing.T) {
	engine := &fakeDictationEngine{transcript: "hello"}
	handler := dictationHandler(t, engine, api.CapabilityDictation)
	var transcript string
	decodeReply(t, dictationCall(t, handler, protocol.OpStop, nil), &transcript)
	if transcript != "hello" {
		t.Fatalf("transcript %q", transcript)
	}
}

func TestDictationStopFailureKeepsItsWords(t *testing.T) {
	engine := &fakeDictationEngine{stopErr: api.ErrNotAvailable}
	handler := dictationHandler(t, engine, api.CapabilityDictation)
	reply := dictationCall(t, handler, protocol.OpStop, nil)
	if reply.Failure == nil || reply.Failure.Code != api.CodeUnavailable {
		t.Fatalf("reply %+v; want an unavailable failure", reply)
	}
}

func TestDictationCancelReachesTheEngine(t *testing.T) {
	engine := &fakeDictationEngine{}
	handler := dictationHandler(t, engine, api.CapabilityDictation)
	if reply := dictationCall(t, handler, protocol.OpCancel, nil); reply.Failure != nil {
		t.Fatalf("cancel failed: %+v", *reply.Failure)
	}
	if engine.cancelled != 1 {
		t.Fatalf("cancelled %d times", engine.cancelled)
	}
}

func TestDictationRecordingsRetryAndDeleteReachTheEngine(t *testing.T) {
	engine := &fakeDictationEngine{
		recordings: []api.SavedDictation{{ID: "dictation-a", CreatedAtMs: 7, SegmentCount: 2}},
		transcript: "retried",
	}
	handler := dictationHandler(t, engine, api.CapabilityDictation)
	var saved []api.SavedDictation
	decodeReply(t, dictationCall(t, handler, protocol.OpRecordings, nil), &saved)
	if len(saved) != 1 || saved[0].ID != "dictation-a" || saved[0].SegmentCount != 2 {
		t.Fatalf("recordings %+v", saved)
	}
	var text string
	decodeReply(t, dictationCall(t, handler, protocol.OpRetry, protocol.DictationRecordingArgs{ID: "dictation-a"}), &text)
	if text != "retried" || len(engine.retried) != 1 || engine.retried[0] != "dictation-a" {
		t.Fatalf("retry text %q, retried %v", text, engine.retried)
	}
	if reply := dictationCall(t, handler, protocol.OpDeleteRecording, protocol.DictationRecordingArgs{ID: "dictation-a"}); reply.Failure != nil {
		t.Fatalf("delete failed: %+v", *reply.Failure)
	}
	if len(engine.deleted) != 1 || engine.deleted[0] != "dictation-a" {
		t.Fatalf("deleted %v", engine.deleted)
	}
}

func TestDictationNeedsTheDictationCapability(t *testing.T) {
	engine := &fakeDictationEngine{transcript: "secret"}
	handler := dictationHandler(t, engine)
	reply := dictationCall(t, handler, protocol.OpStop, nil)
	if reply.Failure == nil || reply.Failure.Code != api.CodePermission {
		t.Fatalf("reply %+v; want the undeclared-capability refusal", reply)
	}
	if reply.Failure != nil && !strings.Contains(reply.Failure.Message, "dictation") {
		t.Fatalf("message %q should name the capability", reply.Failure.Message)
	}
}

func TestDictationWithoutAnEngineIsUnavailable(t *testing.T) {
	handler := dictationHandler(t, nil, api.CapabilityDictation)
	reply := dictationCall(t, handler, protocol.OpStatus, nil)
	if reply.Failure == nil || reply.Failure.Code != api.CodeUnavailable {
		t.Fatalf("reply %+v; want unavailable", reply)
	}
}

func TestDictationRefusesAnUnknownOperation(t *testing.T) {
	handler := dictationHandler(t, &fakeDictationEngine{}, api.CapabilityDictation)
	reply := dictationCall(t, handler, "transcribe-everything", nil)
	if reply.Failure == nil || reply.Failure.Code != api.CodeInvalid {
		t.Fatalf("reply %+v; want an invalid-input failure", reply)
	}
}

// decodeReply reads the data of a successful reply into into.
func decodeReply(t *testing.T, reply protocol.ServiceReply, into any) {
	t.Helper()
	if reply.Failure != nil {
		t.Fatalf("reply failed: %+v", *reply.Failure)
	}
	if err := json.Unmarshal(reply.Data, into); err != nil {
		t.Fatalf("decode %s: %v", reply.Data, err)
	}
}
