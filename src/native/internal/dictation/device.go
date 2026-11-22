package dictation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/dictation/devicemodel"
	"github.com/pablopunk/nvm/native/internal/dictation/webm"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// ErrDeviceUnavailable is an on-device dictation that cannot run here: the model is not downloaded, or this build
// cannot run it. Trying again does not help.
var ErrDeviceUnavailable = errors.New("on-device dictation is not available")

// ErrDeviceFailed is an on-device transcription that failed while it ran. A new attempt starts a fresh model process.
var ErrDeviceFailed = errors.New("on-device transcription failed")

// deviceLinger is how long the on-device model stays loaded after the last dictation that used it, so dictations in
// quick succession do not load it again. After that its memory is given back.
const deviceLinger = 30 * time.Second

// DeviceTranscriber transcribes on this device. Prepare, which runs when the microphone opens, starts loading the
// model, so it is ready by the first pause; the model stays loaded while any dictation or retry holds it, and is closed
// deviceLinger after the last one lets go.
type DeviceTranscriber struct {
	speech platform.SpeechRecognizer
	codec  platform.OpusCodec
	model  *devicemodel.Manager
	linger time.Duration

	mu      sync.Mutex
	session platform.SpeechSession
	holds   int
	held    map[string]bool
	idle    *time.Timer
}

// NewDeviceTranscriber returns a transcriber over the model that model keeps.
func NewDeviceTranscriber(speech platform.SpeechRecognizer, codec platform.OpusCodec, model *devicemodel.Manager) *DeviceTranscriber {
	return &DeviceTranscriber{speech: speech, codec: codec, model: model, linger: deviceLinger, held: map[string]bool{}}
}

// ReadsWholeRecording is true: the model transcribes a dictation in one pass when it stops, as Hex runs this model, so
// a pause never ends a sentence. The model loads while the user speaks, so the pass is all the wait.
func (d *DeviceTranscriber) ReadsWholeRecording() bool { return true }

// Prepare loads the model for dictation operationID, which holds it until Finished.
func (d *DeviceTranscriber) Prepare(_ context.Context, operationID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.held[operationID] {
		return nil
	}
	if _, err := d.acquireLocked(); err != nil {
		return err
	}
	d.held[operationID] = true
	return nil
}

// Transcribe decodes one segment and transcribes it on the device.
func (d *DeviceTranscriber) Transcribe(ctx context.Context, segment Segment) (string, error) {
	pcm, err := d.decode(segment.File)
	if err != nil {
		return "", err
	}
	d.mu.Lock()
	session, err := d.acquireLocked()
	d.mu.Unlock()
	if err != nil {
		return "", err
	}
	defer d.release()
	text, err := session.Transcribe(ctx, pcm)
	if err != nil && ctx.Err() == nil {
		d.discard(session)
		return "", fmt.Errorf("%w: %w", ErrDeviceFailed, err)
	}
	return text, err
}

// CancelPreparation lets go of the model operationID held.
func (d *DeviceTranscriber) CancelPreparation(_ context.Context, operationID string, _ bool) error {
	d.Finished(operationID)
	return nil
}

// Finished lets go of the model operationID held, if it holds it.
func (d *DeviceTranscriber) Finished(operationID string) {
	d.mu.Lock()
	held := d.held[operationID]
	delete(d.held, operationID)
	d.mu.Unlock()
	if held {
		d.release()
	}
}

// acquireLocked opens the model when no session is open, and adds a hold on it.
func (d *DeviceTranscriber) acquireLocked() (platform.SpeechSession, error) {
	if !d.model.Ready() {
		return nil, fmt.Errorf("%w: %w", ErrDeviceUnavailable, devicemodel.ErrNotReady)
	}
	if d.idle != nil {
		d.idle.Stop()
		d.idle = nil
	}
	if d.session == nil {
		session, err := d.speech.Open(d.model.Dir())
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDeviceUnavailable, err)
		}
		d.session = session
	}
	d.holds++
	return d.session, nil
}

// release drops a hold, and closes the model deviceLinger after the last one.
func (d *DeviceTranscriber) release() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.holds--
	if d.holds > 0 || d.session == nil {
		return
	}
	session := d.session
	d.idle = time.AfterFunc(d.linger, func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.holds == 0 && d.session == session {
			_ = session.Close()
			d.session = nil
			d.idle = nil
		}
	})
}

// discard closes a session that failed, so the next request starts a fresh one.
func (d *DeviceTranscriber) discard(session platform.SpeechSession) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.session == session {
		d.session = nil
	}
	_ = session.Close()
}

// decode turns a WebM segment back into 16 kHz mono PCM, without the encoder's lookahead.
func (d *DeviceTranscriber) decode(file []byte) ([]int16, error) {
	track, packets, err := webm.Demux(file)
	if err != nil {
		return nil, err
	}
	if track.SampleRate != SampleRate || track.Channels != Channels {
		return nil, fmt.Errorf("the recording is %d Hz with %d channels; on-device dictation reads 16 kHz mono", track.SampleRate, track.Channels)
	}
	decoder, err := d.codec.NewDecoder(Format())
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	pcm := make([]int16, 0, len(packets)*samplesPerFrame)
	for _, packet := range packets {
		samples, err := decoder.Decode(packet)
		if err != nil {
			return nil, err
		}
		pcm = append(pcm, samples...)
	}
	return pcm[min(track.Lookahead, len(pcm)):], nil
}
