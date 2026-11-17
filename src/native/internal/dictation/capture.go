package dictation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/pablopunk/nvm/native/internal/dictation/webm"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// errStreamEnded is the failure of a microphone that kept ending before it delivered the sound the readiness wait needs.
var errStreamEnded = errors.New("the microphone stream ended before it was ready")

// operation is one dictation from the moment the microphone opens until its text is returned. Only the capture
// goroutine touches its audio until done closes; after that the caller that stopped it owns it.
type operation struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	// captureCtx ends the capture: Stop and Cancel end it, and with it the microphone the capture holds.
	captureCtx  context.Context
	stopCapture context.CancelFunc
	streamMu    sync.Mutex
	stream      platform.MicrophoneStream
	restart     bool
	// reopen opens the microphone again after its stream failed; retry bounds how often, and onRetry hears each try.
	reopen   func(ctx context.Context) (platform.MicrophoneStream, error)
	retry    RetryPolicy
	onRetry  func(attempt int, err error)
	journal  *journal
	encoder  platform.OpusEncoder
	done     chan struct{}
	prepared chan struct{}
	// preparedOK is set before prepared closes; it reports whether the backend accepted the preparation.
	preparedOK   bool
	frames       int
	current      [][]byte
	segments     [][][]byte
	heard        bool
	encodeFailed bool
	failure      error
	onLevel      func(float64)
	levelSink    func(float64, string, func() bool)
	levelPeak    float64
	levelSmooth  float64
	// ready is closed once the readiness tracker hears the device; tracker is nil when the device needs no wait.
	ready   chan struct{}
	tracker *readinessTracker
}

// capture reads the microphone until the dictation stops, frames its samples, encodes each frame and cuts the packets
// into segments, which the journal writes to disk as they grow. A stream that fails for a passing reason, or ends
// while the readiness wait still needs sound, is opened again under the retry policy, so the words after a hiccup are
// recorded too. A partial frame at the end is padded with silence so the last words are kept.
func (op *operation) capture() {
	defer close(op.done)
	frame := make([]int16, samplesPerFrame)
	filled, failures := 0, 0
	for {
		before := op.frames
		var err error
		filled, err = op.readStream(op.currentStream(), frame, filled)
		if op.encodeFailed || op.captureCtx.Err() != nil {
			break
		}
		if op.frames > before {
			failures = 0
		}
		restart := op.takeRestart()
		if err == nil && !restart && op.tracker == nil {
			break
		}
		if err != nil && !transientFailure(err) {
			op.failure = err
			break
		}
		if err == nil && !restart {
			err = errStreamEnded
		}
		if !op.reopenStream(err, restart, &failures) {
			break
		}
	}
	if filled > 0 && !op.encodeFailed {
		clear(frame[filled:])
		op.addFrame(frame)
	}
	op.closeSegment()
}

// readStream reads one stream until it ends. It returns the samples left in the partial frame and nil for a clean end.
func (op *operation) readStream(stream platform.MicrophoneStream, frame []int16, filled int) (int, error) {
	for {
		n, err := stream.Read(frame[filled:])
		filled += n
		if filled == samplesPerFrame {
			if !op.addFrame(frame) {
				return 0, op.failure
			}
			filled = 0
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return filled, nil
			}
			return filled, err
		}
	}
}

// reopenStream opens the microphone again and reports whether the capture goes on. A restart the readiness wait asked
// for opens at once; a failure waits under the retry policy first, and the capture ends with the failure once the
// attempts are spent without a frame in between. A stop meanwhile ends the capture without a failure.
func (op *operation) reopenStream(cause error, restart bool, failures *int) bool {
	for {
		if !restart {
			*failures++
			if *failures >= op.retry.attempts() {
				op.failure = cause
				return false
			}
			if op.onRetry != nil {
				op.onRetry(*failures, cause)
			}
			if op.retry.sleep(op.captureCtx, op.retry.wait(*failures)) != nil {
				return false
			}
		}
		restart = false
		stream, err := op.reopen(op.captureCtx)
		if err == nil {
			if !op.swapStream(stream) {
				_ = stream.Close()
				return false
			}
			return true
		}
		if op.captureCtx.Err() != nil {
			return false
		}
		if !transientFailure(err) {
			op.failure = err
			return false
		}
		cause = err
	}
}

// addFrame encodes one frame and reports whether capture may go on.
func (op *operation) addFrame(frame []int16) bool {
	packet, err := op.encoder.Encode(frame)
	if err != nil {
		op.encodeFailed = true
		op.failure = fmt.Errorf("could not encode the recording: %w", err)
		return false
	}
	op.current = append(op.current, packet)
	op.frames++
	op.heard = op.heard || hasSound(frame)
	op.noteFrame(frame)
	if op.onLevel != nil {
		op.levelPeak = max(op.levelPeak, Peak(frame))
		if op.frames%levelEveryFrames == 0 {
			op.levelSmooth = max(math.Sqrt(op.levelPeak), op.levelSmooth*0.6)
			op.onLevel(op.levelSmooth)
			op.levelPeak = 0
		}
	}
	if len(op.current) == framesPerSegment {
		op.closeSegment()
	} else if op.frames == 1 || op.frames%journalEveryFrames == 0 {
		op.journal.note(len(op.segments), op.current)
	}
	return true
}

// noteFrame feeds the readiness tracker of a device that needs one, and closes ready once the device is heard.
func (op *operation) noteFrame(frame []int16) {
	if op.tracker == nil {
		return
	}
	if op.tracker.consume(float64(op.frames*FrameMillis), hasSound(frame)) {
		op.tracker = nil
		close(op.ready)
	}
}

// closeSegment moves the packets collected so far into a finished segment, and hands it to the journal in full.
func (op *operation) closeSegment() {
	if len(op.current) == 0 {
		return
	}
	op.journal.note(len(op.segments), op.current)
	op.segments = append(op.segments, op.current)
	op.current = nil
}

// hasSpeechAudio reports whether any packet was captured.
func (op *operation) hasSpeechAudio() bool {
	return len(op.segments) > 0
}

// hasAudioToKeep reports whether the recording holds anything but digital silence. A recording with no sound at all is
// the only kind a failure may drop.
func (op *operation) hasAudioToKeep() bool {
	return op.hasSpeechAudio() && op.heard
}

// track is the WebM track the segments of op are muxed with.
func (op *operation) track() webm.Track {
	return webm.Track{SampleRate: SampleRate, Channels: Channels, Lookahead: op.encoder.Lookahead()}
}

// endCapture stops the capture for good: the stream closes and no other one opens.
func (op *operation) endCapture() {
	op.stopCapture()
	if stream := op.currentStream(); stream != nil {
		_ = stream.Close()
	}
}

// restartStream closes the current stream so the capture opens the microphone again.
func (op *operation) restartStream() {
	op.streamMu.Lock()
	op.restart = true
	stream := op.stream
	op.streamMu.Unlock()
	if stream != nil {
		_ = stream.Close()
	}
}

func (op *operation) currentStream() platform.MicrophoneStream {
	op.streamMu.Lock()
	defer op.streamMu.Unlock()
	return op.stream
}

func (op *operation) takeRestart() bool {
	op.streamMu.Lock()
	defer op.streamMu.Unlock()
	restart := op.restart
	op.restart = false
	return restart
}

// swapStream installs a reopened stream and reports false when the capture ended meanwhile.
func (op *operation) swapStream(stream platform.MicrophoneStream) bool {
	op.streamMu.Lock()
	defer op.streamMu.Unlock()
	if op.captureCtx.Err() != nil {
		return false
	}
	op.stream = stream
	return true
}
