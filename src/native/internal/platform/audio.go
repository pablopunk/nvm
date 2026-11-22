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

// OpusCodec is the intent "encode PCM as Opus packets, and decode them back". The Opus library is loaded at run time;
// where it is missing the constructors return ErrUnsupported and the caller explains what to install.
type OpusCodec interface {
	// NewEncoder returns an encoder for format, tuned for speech.
	NewEncoder(format AudioFormat) (OpusEncoder, error)
	// NewDecoder returns a decoder that answers PCM in format.
	NewDecoder(format AudioFormat) (OpusDecoder, error)
}

// OpusDecoder turns one Opus packet at a time back into interleaved PCM.
type OpusDecoder interface {
	// Decode returns the samples of one packet.
	Decode(packet []byte) ([]int16, error)
	// Close releases the decoder.
	Close() error
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

// SpeechRecognizer is the intent "turn speech into text on this device". Open returns ErrUnsupported where the
// runtime the model needs is missing.
type SpeechRecognizer interface {
	// Open starts loading the model in dir and returns at once, so the load runs while the user speaks. The model's
	// memory is held until the session is closed.
	Open(dir string) (SpeechSession, error)
}

// SpeechSession is one loaded model. It is safe for concurrent use; requests are answered one at a time.
type SpeechSession interface {
	// Transcribe returns the text of 16 kHz mono PCM. The first call waits for the model to finish loading.
	Transcribe(ctx context.Context, pcm []int16) (string, error)
	// Close gives the model's memory back. A request in flight fails.
	Close() error
}
