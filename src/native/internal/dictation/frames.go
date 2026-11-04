package dictation

import (
	"math"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Capture geometry. Speech is recorded mono at 16 kHz, which the transcription model decodes without loss, in
// 20 ms Opus frames, and a segment is one minute long, the same split the desktop app makes.
const (
	SampleRate       = 16000
	Channels         = 1
	FrameMillis      = 20
	SegmentMillis    = 60_000
	framesPerSegment = SegmentMillis / FrameMillis
	samplesPerFrame  = SampleRate * FrameMillis / 1000 * Channels
	levelEveryFrames = 5
	maxLevel         = 1.0
)

// Format is the PCM the microphone is opened with.
func Format() platform.AudioFormat {
	return platform.AudioFormat{SampleRate: SampleRate, Channels: Channels}
}

// Level is the root-mean-square of a frame, scaled to 0..1 for the indicator's meter. A quiet room reads near zero
// and full-scale speech near one; the scale is linear so the meter moves with the voice.
func Level(pcm []int16) float64 {
	if len(pcm) == 0 {
		return 0
	}
	var sum float64
	for _, sample := range pcm {
		value := float64(sample) / math.MaxInt16
		sum += value * value
	}
	return math.Min(maxLevel, math.Sqrt(sum/float64(len(pcm))))
}
