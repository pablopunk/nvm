//go:build darwin || linux

package real

import (
	"math"
	"testing"

	"github.com/ebitengine/purego"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// decodeOpus round-trips packets through the same libopus, so the encoder's output is checked by the library's own
// decoder rather than by byte patterns.
func decodeOpus(t *testing.T, packets [][]byte, sampleRate, channels int) []int16 {
	t.Helper()
	handle, err := openFirstLibrary(libopusNames)
	if err != nil {
		t.Skipf("libopus is not loadable: %v", err)
	}
	var create func(sampleRate, channels int32, errOut *int32) uintptr
	var decode func(state uintptr, data *byte, length int32, pcm *int16, frameSize int32, fec int32) int32
	purego.RegisterLibFunc(&create, handle, "opus_decoder_create")
	purego.RegisterLibFunc(&decode, handle, "opus_decode")
	var code int32
	state := create(int32(sampleRate), int32(channels), &code)
	if code != 0 || state == 0 {
		t.Fatalf("opus_decoder_create failed with %d", code)
	}
	var out []int16
	frame := make([]int16, sampleRate/50*channels)
	for _, packet := range packets {
		n := decode(state, &packet[0], int32(len(packet)), &frame[0], int32(len(frame)/channels), 0)
		if n < 0 {
			t.Fatalf("opus_decode returned %d", n)
		}
		out = append(out, frame[:int(n)*channels]...)
	}
	return out
}

func TestLibopusEncodesSpeechThatDecodesBackToTheSameTone(t *testing.T) {
	codec := opusCodec{}
	encoder, err := codec.NewEncoder(platform.AudioFormat{SampleRate: 16000, Channels: 1})
	if err != nil {
		t.Skipf("libopus is not available: %v", err)
	}
	defer func() { _ = encoder.Close() }()
	if encoder.Lookahead() <= 0 {
		t.Fatalf("lookahead = %d, want the encoder's delay", encoder.Lookahead())
	}
	var packets [][]byte
	for frame := 0; frame < 50; frame++ {
		pcm := make([]int16, 320)
		for i := range pcm {
			sample := 0.5 * math.Sin(2*math.Pi*440*float64(frame*320+i)/16000)
			pcm[i] = int16(sample * math.MaxInt16)
		}
		packet, err := encoder.Encode(pcm)
		if err != nil {
			t.Fatalf("Encode frame %d: %v", frame, err)
		}
		if len(packet) == 0 || len(packet) > opusMaxPacketBytes {
			t.Fatalf("packet %d has %d bytes", frame, len(packet))
		}
		packets = append(packets, packet)
	}
	decoded := decodeOpus(t, packets, 16000, 1)
	var energy float64
	for _, sample := range decoded[len(decoded)/2:] {
		energy += float64(sample) * float64(sample)
	}
	if rms := math.Sqrt(energy / float64(len(decoded)/2)); rms < 4000 {
		t.Fatalf("decoded RMS = %.0f, want a clear tone; the encoder output is not Opus speech", rms)
	}
}

func TestLibopusRejectsAFrameThatIsNotWholeSamples(t *testing.T) {
	encoder, err := opusCodec{}.NewEncoder(platform.AudioFormat{SampleRate: 16000, Channels: 2})
	if err != nil {
		t.Skipf("libopus is not available: %v", err)
	}
	defer func() { _ = encoder.Close() }()
	if _, err := encoder.Encode(make([]int16, 321)); err == nil {
		t.Fatalf("a stereo frame with an odd sample count was encoded")
	}
}
