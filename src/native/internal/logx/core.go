package logx

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// flushBytes is the pending size that forces a write to the file before the next tick.
const flushBytes = 64 << 10

// core is the state every child logger shares: the ring, the pending batch, and the file bookkeeping.
// Entries are written to the file in batches, because the platform FS port can only replace a whole
// file; each batch costs one rewrite of the active file, so the flush interval bounds the write rate.
//
// Locking: mu guards the ring, pending, dropped and the stderr writes, and is held only briefly.
// ioMu serializes file work (flush, rotation) and guards active, the bytes the active file holds.
type core struct {
	now        func() time.Time
	files      platform.FS
	activePath string
	opts       Options
	level      Level
	redactor   *Redactor
	stderr     io.Writer

	mu      sync.Mutex
	ring    *ring
	pending []byte
	dropped int

	ioMu   sync.Mutex
	active []byte

	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
}

// newCore builds the shared state without touching the disk. A nil files keeps entries in the ring and
// on stderr only.
func newCore(now func() time.Time, files platform.FS, dir string, opts Options) *core {
	return &core{
		now:        now,
		files:      files,
		activePath: joinPath(dir, opts.FileName),
		opts:       opts,
		level:      opts.Level,
		redactor:   opts.Redactor,
		stderr:     opts.Stderr,
		ring:       newRing(RingCapacity),
		done:       make(chan struct{}),
	}
}

// write stores e in the ring, mirrors it to stderr, and queues it for the file. An error or a full
// batch flushes at once; otherwise the flusher writes it on the next tick.
func (c *core) write(e Entry) {
	line, err := e.encode()
	c.mu.Lock()
	c.ring.push(e)
	if err != nil {
		c.dropped++
		c.mu.Unlock()
		return
	}
	if c.stderr != nil {
		_, _ = c.stderr.Write(line)
	}
	if c.files == nil {
		c.mu.Unlock()
		return
	}
	c.pending = append(c.pending, line...)
	urgent := e.Level >= Error || len(c.pending) >= flushBytes
	c.mu.Unlock()
	if urgent {
		_ = c.flush()
	}
}

// flush writes the pending batch to the active file. A failed write drops the batch, counts its lines
// and returns the error, so memory stays bounded when the disk is full.
func (c *core) flush() error {
	c.ioMu.Lock()
	defer c.ioMu.Unlock()
	c.mu.Lock()
	batch := c.pending
	c.pending = nil
	c.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	if err := c.appendBatch(batch); err != nil {
		c.mu.Lock()
		c.dropped += bytes.Count(batch, []byte{'\n'})
		c.mu.Unlock()
		return err
	}
	return nil
}

// startFlusher writes pending entries on every tick of the clock until stop is called. It runs under
// safego, so a panic is reported and contained, and close(done) happens exactly once on exit.
func (c *core) startFlusher(clock platform.Clock, interval time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	safego.Go("logx.flush", func(ctx context.Context) {
		c.flushLoop(ctx, clock, interval)
	}, safego.WithContext(ctx), safego.WithClock(clock))
}

func (c *core) flushLoop(ctx context.Context, clock platform.Clock, interval time.Duration) {
	ticker := clock.NewTicker(interval)
	defer ticker.Stop()
	defer c.closeOnce.Do(func() { close(c.done) })
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			_ = c.flush()
		}
	}
}

// stop ends the flusher and waits for it. It is safe to call more than once.
func (c *core) stop() {
	if c.cancel == nil {
		return
	}
	c.cancel()
	<-c.done
}
