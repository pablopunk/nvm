package platform

import "context"

// AudioFormat describes the PCM a microphone delivers: signed 16-bit little-endian samples at a fixed rate and
// channel count.
type AudioFormat struct {
	SampleRate int
	Channels   int
}

// MicrophoneDevice is an input device the user can pick. ID is what Open takes; Title is what the UI shows.
type MicrophoneDevice struct {
	ID      string
	Title   string
	Default bool
}

// Microphone is the intent "list the input devices and record PCM from one". Capture needs the capability
// microphone.capture; where the OS cannot provide it, Devices and Open return ErrUnsupported. Open returns
// ErrNotFound when the device is gone and ErrDenied when the OS refused the microphone.
type Microphone interface {
	// Devices lists the input devices, the default one marked.
	Devices() ([]MicrophoneDevice, error)
	// Open starts recording from deviceID, or the default device when deviceID is "" or "default". The
	// stream ends when it is closed, or when ctx is done.
	Open(ctx context.Context, deviceID string, format AudioFormat) (MicrophoneStream, error)
}

// MicrophoneStream delivers PCM as it is captured.
type MicrophoneStream interface {
	// Read fills pcm with the next samples and returns how many it wrote. It blocks until samples are available,
	// and returns io.EOF once the stream is closed or its source ended.
	Read(pcm []int16) (int, error)
	// Close stops the capture and unblocks a pending Read.
	Close() error
}

// OpusCodec is the intent "encode PCM as Opus packets". The Opus library is loaded at run time; where it is
// missing the constructor returns ErrUnsupported and the caller explains what to install.
type OpusCodec interface {
	// NewEncoder returns an encoder for format, tuned for speech.
	NewEncoder(format AudioFormat) (OpusEncoder, error)
}

// OpusEncoder encodes fixed-size frames of 20 ms. Each Encode call takes exactly one frame of
// format.SampleRate/50 samples per channel and returns one Opus packet.
type OpusEncoder interface {
	// Encode returns the packet for one frame of interleaved samples.
	Encode(pcm []int16) ([]byte, error)
	// Lookahead is the encoder delay in samples at the input rate, which the container records as pre-skip.
	Lookahead() int
	// Close releases the encoder.
	Close() error
}
