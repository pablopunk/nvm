package dictation

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/dictation/webm"
)

// journalEveryFrames is how often the open segment is written while the user speaks: once per second of audio.
const journalEveryFrames = 1000 / FrameMillis

// journal writes one dictation's audio to the recorder while it is captured: the first frame at once, the open segment
// about every second, and each finished segment in full, so a crash or a quit loses at most the last second. It writes
// on its own goroutine, so a slow disk never stalls the capture; finish waits for it and writes whatever is missing.
// A nil journal keeps nothing.
type journal struct {
	recorder Recorder
	id       string
	track    webm.Track
	warn     func(event string, err error)

	mu      sync.Mutex
	pending map[int][][]byte
	// written is how many packets of each segment are on disk.
	written map[int]int
	failed  bool
	closed  bool
	// saved closes after every write, then is replaced, so awaitSaved can wait for the next one.
	saved chan struct{}
	wake  chan struct{}
	done  chan struct{}
}

// newJournal starts the writer of dictation id's audio.
func newJournal(recorder Recorder, id string, track webm.Track, warn func(string, error)) *journal {
	j := &journal{
		recorder: recorder, id: id, track: track, warn: warn,
		pending: map[int][][]byte{}, written: map[int]int{},
		saved: make(chan struct{}), wake: make(chan struct{}, 1), done: make(chan struct{}),
	}
	go j.run()
	return j
}

// note queues the packets of segment index for the writer, replacing an older copy that was not written yet.
func (j *journal) note(index int, packets [][]byte) {
	if j == nil {
		return
	}
	j.mu.Lock()
	j.pending[index] = slices.Clone(packets)
	j.mu.Unlock()
	j.signal()
}

func (j *journal) signal() {
	select {
	case j.wake <- struct{}{}:
	default:
	}
}

// run writes the queued segments until the journal is closed and nothing is left.
func (j *journal) run() {
	defer close(j.done)
	for {
		j.mu.Lock()
		for len(j.pending) == 0 && !j.closed {
			j.mu.Unlock()
			<-j.wake
			j.mu.Lock()
		}
		batch := j.pending
		j.pending = map[int][][]byte{}
		closed := j.closed
		j.mu.Unlock()
		for _, index := range slices.Sorted(maps.Keys(batch)) {
			j.write(index, batch[index])
		}
		if closed && len(batch) == 0 {
			return
		}
	}
}

// write saves one segment and remembers how much of it is on disk. The first failure is logged; later ones are not, so
// a full disk does not log once a second.
func (j *journal) write(index int, packets [][]byte) bool {
	err := j.recorder.SaveSegment(j.id, index, webm.Mux(j.track, packets, FrameMillis))
	j.mu.Lock()
	defer j.mu.Unlock()
	close(j.saved)
	j.saved = make(chan struct{})
	if err != nil {
		if !j.failed && j.warn != nil {
			j.warn("dictation.recording-save-failed", err)
		}
		j.failed = true
		return false
	}
	j.written[index] = max(j.written[index], len(packets))
	return true
}

// awaitSaved waits until count packets of segment index are on disk, so a segment never reaches the backend before
// its audio is kept. It also returns once the disk has failed (sending beats losing the words twice), the writer has
// stopped, or ctx ends. A nil journal keeps nothing and returns at once.
func (j *journal) awaitSaved(ctx context.Context, index, count int) {
	if j == nil {
		return
	}
	for {
		j.mu.Lock()
		if j.written[index] >= count || j.failed {
			j.mu.Unlock()
			return
		}
		next := j.saved
		j.mu.Unlock()
		select {
		case <-next:
		case <-j.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// finish stops the writer and writes every segment that is not on disk in full. It reports whether the whole recording
// is now on disk, which is what lets a failure say the recording was saved.
func (j *journal) finish(segments [][][]byte) bool {
	if j == nil {
		return false
	}
	j.mu.Lock()
	j.closed = true
	j.mu.Unlock()
	j.signal()
	<-j.done
	complete := len(segments) > 0
	for index, packets := range segments {
		j.mu.Lock()
		onDisk := j.written[index] == len(packets)
		j.mu.Unlock()
		if !onDisk && !j.write(index, packets) {
			complete = false
		}
	}
	return complete
}
