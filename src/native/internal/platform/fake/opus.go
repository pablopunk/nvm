package fake

import (
	"encoding/binary"

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
