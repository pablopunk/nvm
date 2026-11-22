package fake

import (
	"encoding/binary"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpusCodec is a platform.OpusCodec with no Opus library. Each packet is the frame's length and its first sample,
// so tests can check frame order and sizes without decoding audio.
type OpusCodec struct {
	lookahead int
	faults    faultSet
}

var _ platform.OpusCodec = (*OpusCodec)(nil)

// NewOpusCodec returns a codec whose encoders report a 312-sample lookahead, the libopus value at 16 kHz.
func NewOpusCodec() *OpusCodec {
	return &OpusCodec{lookahead: 312}
}

// FailNext makes the next NewEncoder call return err.
func (c *OpusCodec) FailNext(op string, err error) {
	c.faults.failNext(op, err)
}

// NewEncoder returns a fake encoder for format.
func (c *OpusCodec) NewEncoder(format platform.AudioFormat) (platform.OpusEncoder, error) {
	if err := c.faults.take("opus.new"); err != nil {
		return nil, err
	}
	return &opusEncoder{lookahead: c.lookahead}, nil
}

type opusEncoder struct {
	lookahead int
}

// Encode returns a fake packet: the frame length as a little-endian uint32, then the first sample.
func (e *opusEncoder) Encode(pcm []int16) ([]byte, error) {
	packet := make([]byte, 6)
	binary.LittleEndian.PutUint32(packet, uint32(len(pcm)))
	if len(pcm) > 0 {
		binary.LittleEndian.PutUint16(packet[4:], uint16(pcm[0]))
	}
	return packet, nil
}

// Lookahead returns the scripted lookahead.
func (e *opusEncoder) Lookahead() int { return e.lookahead }

// Close is a no-op.
func (e *opusEncoder) Close() error { return nil }

// NewDecoder returns a decoder of the fake encoder's packets.
func (c *OpusCodec) NewDecoder(format platform.AudioFormat) (platform.OpusDecoder, error) {
	if err := c.faults.take("opus.decoder"); err != nil {
		return nil, err
	}
	return opusDecoder{}, nil
}

type opusDecoder struct{}

// Decode turns a fake packet back into a frame of its length, every sample the frame's first one.
func (opusDecoder) Decode(packet []byte) ([]int16, error) {
	if len(packet) != 6 {
		return nil, fmt.Errorf("a fake Opus packet holds 6 bytes, got %d", len(packet))
	}
	pcm := make([]int16, binary.LittleEndian.Uint32(packet))
	for i := range pcm {
		pcm[i] = int16(binary.LittleEndian.Uint16(packet[4:]))
	}
	return pcm, nil
}

// Close is a no-op.
func (opusDecoder) Close() error { return nil }
