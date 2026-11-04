//go:build darwin || linux

package real

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// libopusNames are the library names tried in order. The first one that loads wins; the Linux names are the
// system packages, and the macOS paths are Homebrew's, since a dynamic loader does not search them by default.
var libopusNames = []string{
	"libopus.so.0",
	"libopus.so",
	"/opt/homebrew/lib/libopus.dylib",
	"/usr/local/lib/libopus.dylib",
	"libopus.dylib",
}

const (
	opusApplicationVOIP = 2048
	opusGetLookahead    = 4027
	opusOK              = 0
	// opusMaxPacketBytes bounds one packet; libopus recommends 4000 bytes for any frame.
	opusMaxPacketBytes = 4000
)

// libopus holds the functions of the loaded library. It is loaded once, on first use.
type libopus struct {
	encoderCreate  func(sampleRate, channels, application int32, errOut *int32) uintptr
	encode         func(encoder uintptr, pcm *int16, frameSize int32, data *byte, maxBytes int32) int32
	encoderControl func(encoder uintptr, request int32, value *int32) int32
	encoderDestroy func(encoder uintptr)
}

var (
	libopusOnce sync.Once
	libopusLib  *libopus
	libopusErr  error
)

func loadLibopus() (*libopus, error) {
	libopusOnce.Do(func() {
		handle, err := openFirstLibrary(libopusNames)
		if err != nil {
			libopusErr = fmt.Errorf("%w: libopus is not installed (%v)", platform.ErrUnsupported, err)
			return
		}
		lib := &libopus{}
		if err := bindLibopus(handle, lib); err != nil {
			libopusErr = fmt.Errorf("%w: libopus is missing a function: %v", platform.ErrUnsupported, err)
			return
		}
		libopusLib = lib
	})
	return libopusLib, libopusErr
}

func openFirstLibrary(names []string) (uintptr, error) {
	var lastErr error = errors.New("no library names")
	for _, name := range names {
		handle, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			return handle, nil
		}
		lastErr = err
	}
	return 0, lastErr
}

// bindLibopus resolves each symbol before registering it, because RegisterLibFunc panics on a missing one.
func bindLibopus(handle uintptr, lib *libopus) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v", recovered)
		}
	}()
	bindings := []struct {
		name string
		bind func()
	}{
		{"opus_encoder_create", func() { purego.RegisterLibFunc(&lib.encoderCreate, handle, "opus_encoder_create") }},
		{"opus_encode", func() { purego.RegisterLibFunc(&lib.encode, handle, "opus_encode") }},
		{"opus_encoder_ctl", func() { purego.RegisterLibFunc(&lib.encoderControl, handle, "opus_encoder_ctl") }},
		{"opus_encoder_destroy", func() { purego.RegisterLibFunc(&lib.encoderDestroy, handle, "opus_encoder_destroy") }},
	}
	for _, binding := range bindings {
		if _, symErr := purego.Dlsym(handle, binding.name); symErr != nil {
			return symErr
		}
		binding.bind()
	}
	return nil
}

// opusCodec is the production platform.OpusCodec, backed by the system libopus.
type opusCodec struct{}

// NewEncoder creates a libopus encoder in VOIP mode for format.
func (opusCodec) NewEncoder(format platform.AudioFormat) (platform.OpusEncoder, error) {
	lib, err := loadLibopus()
	if err != nil {
		return nil, err
	}
	var code int32
	state := lib.encoderCreate(int32(format.SampleRate), int32(format.Channels), opusApplicationVOIP, &code)
	if code != opusOK || state == 0 {
		return nil, fmt.Errorf("libopus refused the encoder settings (error %d)", code)
	}
	encoder := &libopusEncoder{lib: lib, state: state, format: format}
	var lookahead int32
	if lib.encoderControl(state, opusGetLookahead, &lookahead) == opusOK {
		encoder.lookahead = int(lookahead)
	}
	return encoder, nil
}

// libopusEncoder encodes one 20 ms frame per call. The state is freed by Close.
type libopusEncoder struct {
	lib       *libopus
	state     uintptr
	format    platform.AudioFormat
	lookahead int
}

// Encode returns the Opus packet for one frame. The frame must hold exactly 20 ms of interleaved samples.
func (e *libopusEncoder) Encode(pcm []int16) ([]byte, error) {
	if len(pcm) == 0 || len(pcm)%e.format.Channels != 0 {
		return nil, fmt.Errorf("an Opus frame needs whole interleaved samples, got %d", len(pcm))
	}
	frameSize := int32(len(pcm) / e.format.Channels)
	out := make([]byte, opusMaxPacketBytes)
	n := e.lib.encode(e.state, &pcm[0], frameSize, &out[0], opusMaxPacketBytes)
	if n < 0 {
		return nil, fmt.Errorf("libopus failed to encode a frame (error %d)", n)
	}
	return out[:n], nil
}

// Lookahead is libopus's encoder delay in samples at the input rate.
func (e *libopusEncoder) Lookahead() int { return e.lookahead }

// Close frees the libopus state. Encode must not be called afterwards.
func (e *libopusEncoder) Close() error {
	if e.state != 0 {
		e.lib.encoderDestroy(e.state)
		e.state = 0
	}
	return nil
}
