package real

import "bytes"

// limitedBuffer keeps the first limit bytes written to it and reports the rest as discarded. It always
// accepts the whole write, so the child process never sees a broken pipe because of the limit.
type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

// Write stores as much of p as fits under the limit.
func (b *limitedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.buf.Len()
	switch {
	case room <= 0:
		if len(p) > 0 {
			b.truncated = true
		}
	case len(p) > room:
		b.buf.Write(p[:room])
		b.truncated = true
	default:
		b.buf.Write(p)
	}
	return len(p), nil
}
