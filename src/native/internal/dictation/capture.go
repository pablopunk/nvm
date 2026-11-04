package dictation

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// operation is one dictation from the moment the microphone opens until its text is returned. Only the capture
// goroutine touches it until done closes; after that the caller that stopped it owns it.
type operation struct {
	id       string
	ctx      context.Context
	cancel   context.CancelFunc
	stream   platform.MicrophoneStream
	encoder  platform.OpusEncoder
	done     chan struct{}
	prepared chan struct{}
	frames   int
	current  [][]byte
	segments [][][]byte
	failure  error
	onLevel  func(float64)
}

// capture reads the microphone until the stream ends, frames its samples, encodes each frame and cuts the packets
// into segments. A partial frame at the end is padded with silence so the last words are kept.
func (op *operation) capture() {
	defer close(op.done)
	frame := make([]int16, samplesPerFrame)
	filled := 0
	for {
		n, err := op.stream.Read(frame[filled:])
		filled += n
		for filled == samplesPerFrame {
			if !op.addFrame(frame) {
				return
			}
			filled = 0
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				op.failure = err
				return
			}
			break
		}
	}
	if filled > 0 {
		clear(frame[filled:])
		op.addFrame(frame)
	}
	op.closeSegment()
}

// addFrame encodes one frame and reports whether capture may go on.
func (op *operation) addFrame(frame []int16) bool {
	packet, err := op.encoder.Encode(frame)
	if err != nil {
		op.failure = fmt.Errorf("could not encode the recording: %w", err)
		return false
	}
	op.current = append(op.current, packet)
	op.frames++
	if op.onLevel != nil && op.frames%levelEveryFrames == 0 {
		op.onLevel(Level(frame))
	}
	if len(op.current) == framesPerSegment {
		op.closeSegment()
	}
	return true
}

// closeSegment moves the packets collected so far into a finished segment.
func (op *operation) closeSegment() {
	if len(op.current) == 0 {
		return
	}
	op.segments = append(op.segments, op.current)
	op.current = nil
}

// hasSpeechAudio reports whether any packet was captured.
func (op *operation) hasSpeechAudio() bool {
	return len(op.segments) > 0
}
