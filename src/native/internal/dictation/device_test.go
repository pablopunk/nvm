package dictation

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/dictation/devicemodel"
	"github.com/pablopunk/nvm/native/internal/dictation/webm"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// deviceRig is a device transcriber over fakes, its model on disk unless the test removes it.
type deviceRig struct {
	files  *fake.FS
	speech *fake.SpeechRecognizer
	model  *devicemodel.Manager
	device *DeviceTranscriber
}

func newDeviceRig(t *testing.T, downloaded bool) deviceRig {
	t.Helper()
	files := fake.NewFS()
	manifest := devicemodel.Manifest{Name: "tiny", Title: "Tiny", Files: []devicemodel.File{{Name: "model.onnx", Size: 3}}}
	model := devicemodel.New(files, fake.NewDownloads(), "/models", manifest)
	if downloaded {
		_ = files.MkdirAll("/models/tiny", 0o755)
		if err := files.WriteFile("/models/tiny/model.onnx", []byte("abc"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	speech := fake.NewSpeechRecognizer()
	device := NewDeviceTranscriber(speech, fake.NewOpusCodec(), model)
	device.linger = time.Millisecond
	return deviceRig{files: files, speech: speech, model: model, device: device}
}

// segmentOf encodes frames of one sample each, as the fake codec does, into the WebM file a segment carries.
func segmentOf(frames int, lookahead int) []byte {
	encoder, _ := fake.NewOpusCodec().NewEncoder(Format())
	packets := make([][]byte, frames)
	for i := range packets {
		frame := make([]int16, samplesPerFrame)
		for j := range frame {
			frame[j] = int16(i + 1)
		}
		packets[i], _ = encoder.Encode(frame)
	}
	return webm.Mux(webm.Track{SampleRate: SampleRate, Channels: Channels, Lookahead: lookahead}, packets, FrameMillis)
}

func TestDeviceTranscriberDecodesTheSegmentAndDropsTheEncoderDelay(t *testing.T) {
	rig := newDeviceRig(t, true)
	text, err := rig.device.Transcribe(context.Background(), Segment{OperationID: "op", File: segmentOf(3, 100)})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	heard := rig.speech.Heard()
	if len(heard) != 1 || len(heard[0]) != 3*samplesPerFrame-100 || text != "heard 860 samples" {
		t.Fatalf("heard %d requests (%v), text %q; want the 3 frames less the 100-sample delay", len(heard), lengths(heard), text)
	}
	if heard[0][0] != 1 || heard[0][len(heard[0])-1] != 3 {
		t.Fatalf("samples run %d..%d; want the frames in order", heard[0][0], heard[0][len(heard[0])-1])
	}
}

func TestTheModelLoadsOnPrepareServesTheDictationAndUnloadsAfterIt(t *testing.T) {
	rig := newDeviceRig(t, true)
	if err := rig.device.Prepare(context.Background(), "op"); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	for range 2 {
		if _, err := rig.device.Transcribe(context.Background(), Segment{OperationID: "op", File: segmentOf(2, 0)}); err != nil {
			t.Fatal(err)
		}
	}
	if opened := rig.speech.Opened(); len(opened) != 1 || opened[0] != filepath.Join("/models", "tiny") {
		t.Fatalf("opened %v; want the model loaded once, from its folder", opened)
	}
	time.Sleep(20 * time.Millisecond)
	if rig.speech.OpenSessions() != 1 {
		t.Fatal("the model was unloaded while the dictation still held it")
	}
	rig.device.Finished("op")
	waitFor(t, func() bool { return rig.speech.OpenSessions() == 0 }, "the model to unload after the dictation")
}

func TestADictationOnTheDeviceIsTranscribedWholeWhenItStops(t *testing.T) {
	rig := newDeviceRig(t, true)
	h := newHarness(t)
	audio := slices.Concat(voice(6), make([]int16, SampleRate*6/10), voice(2))
	h.mic.Queue(audio...)
	if err := h.service.StartOn(rig.device, "default", nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-currentOperation(h.service).done
	if len(rig.speech.Heard()) != 0 {
		t.Fatal("the device transcribed a piece before the dictation stopped")
	}
	if rig.speech.OpenSessions() != 1 {
		t.Fatal("the model was not loading while the user spoke")
	}
	text, err := h.service.Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if heard := rig.speech.Heard(); len(heard) != 1 || len(heard[0]) < len(audio) || strings.Count(text, "heard") != 1 {
		t.Fatalf("text = %q after %d requests; want the whole recording read once from its raw samples", text, len(heard))
	}
	if len(h.transcriber.ids()) != 0 {
		t.Fatal("an on-device dictation reached the cloud transcriber")
	}
	waitFor(t, func() bool { return rig.speech.OpenSessions() == 0 }, "the model to unload after Stop")
}

func TestADictationOnAModelNotDownloadedFailsAtOnceWithoutRetries(t *testing.T) {
	rig := newDeviceRig(t, false)
	if err := rig.device.Prepare(context.Background(), "op"); !errors.Is(err, devicemodel.ErrNotReady) {
		t.Fatalf("Prepare = %v; want the model reported missing", err)
	}
	if transientFailure(&SavedError{Err: errors.Join(ErrDeviceUnavailable)}) || transientFailure(ErrDeviceUnavailable) {
		t.Fatal("a missing model would be retried")
	}
	if got := FailureMessage(errors.Join(ErrDeviceUnavailable, devicemodel.ErrNotReady)); !strings.Contains(got, "Download the on-device model") {
		t.Fatalf("message = %q; want the user told to download the model", got)
	}
	if len(rig.speech.Opened()) != 0 {
		t.Fatal("a model that is not on disk was opened")
	}
}

func TestAFailedRequestStartsAFreshModelProcess(t *testing.T) {
	rig := newDeviceRig(t, true)
	rig.speech.FailNext("speech.transcribe", errors.New("child exited"))
	_, err := rig.device.Transcribe(context.Background(), Segment{OperationID: "op", File: segmentOf(1, 0)})
	if !errors.Is(err, ErrDeviceFailed) || !transientFailure(err) {
		t.Fatalf("err = %v; want a passing on-device failure", err)
	}
	if _, err := rig.device.Transcribe(context.Background(), Segment{OperationID: "op", File: segmentOf(1, 0)}); err != nil {
		t.Fatalf("second Transcribe: %v", err)
	}
	if opened := rig.speech.Opened(); len(opened) != 2 {
		t.Fatalf("opened %d sessions; want a fresh one after the failure", len(opened))
	}
}

func lengths(heard [][]int16) []int {
	out := make([]int, len(heard))
	for i, pcm := range heard {
		out[i] = len(pcm)
	}
	return out
}

func waitFor(t *testing.T, done func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestALongOnDeviceDictationIsReadWholeThenItsEndIsChecked(t *testing.T) {
	rig := newDeviceRig(t, true)
	h := newHarness(t)
	h.mic.Queue(voice(61)...)
	if err := h.service.StartOn(rig.device, "default", nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := h.service.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	heard := rig.speech.Heard()
	if len(heard) != 2 || len(heard[0]) < 61*SampleRate {
		t.Fatalf("heard %v samples; want all 61 s read whole, then the end read again", lengths(heard))
	}
	if tail := len(heard[1]); tail < tailCheckMillis*SampleRate/1000 || tail > (tailCheckMillis+tailSearchMillis)*SampleRate/1000 {
		t.Fatalf("end check read %d samples; want 15 to 20 s", tail)
	}
}

func TestARetryOnTheDeviceReadsTheSavedSegmentsAcrossTheirCut(t *testing.T) {
	rig := newDeviceRig(t, true)
	h := newHarness(t)
	codec := fake.NewOpusCodec()
	encoder, _ := codec.NewEncoder(Format())
	track := webm.Track{SampleRate: SampleRate, Channels: Channels}
	segment := func(frames int) []byte {
		packets := make([][]byte, frames)
		for i := range packets {
			packets[i], _ = encoder.Encode(make([]int16, samplesPerFrame))
		}
		return webm.Mux(track, packets, FrameMillis)
	}
	text, err := h.service.retryFiles(context.Background(), rig.device, [][]byte{segment(3000), segment(500)})
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if heard := rig.speech.Heard(); len(heard) != 2 || len(heard[0]) != 3500*samplesPerFrame || text == "" {
		t.Fatalf("heard %v samples, text %q; want the 70 s of both saved segments read whole, then the end checked", lengths(heard), text)
	}
}
