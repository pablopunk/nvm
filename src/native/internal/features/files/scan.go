package files

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// scanWorkers is the number of folders read at once when the configuration does not say otherwise. The FS port
// is safe for concurrent use; more workers stop helping once the disk is the bottleneck.
const scanWorkers = 4

// scanConfig is what one walk needs from the index configuration.
type scanConfig struct {
	// maxDepth is the number of folder levels below a root that are read: files directly in the root are at
	// depth zero, as file-index.ts counts them.
	maxDepth      int
	maxEntries    int
	rules         IgnoreRules
	includeHidden bool
	// statFiles asks for size and times. Name-only sorts skip the per-file metadata read.
	statFiles bool
	workers   int
}

// scanStart is a folder a walk begins from. rel is its path relative to the root that contains it, which anchored
// ignore patterns match against; remaining is how many more levels may be read below it.
type scanStart struct {
	dir       string
	rel       string
	remaining int
}

// scanCounters are read by Status while a scan runs.
type scanCounters struct {
	dirs  atomic.Int64
	files atomic.Int64
}

// scanResult is what a walk found.
type scanResult struct {
	records    []record
	unreadable []string
	truncated  bool
}

// scanner holds the shared state of one walk. Workers collect records locally and merge them once at the end.
type scanner struct {
	fsys     platform.FS
	cfg      scanConfig
	queue    *workQueue
	counters *scanCounters
	total    atomic.Int64
	halted   atomic.Bool

	mu         sync.Mutex
	records    []record
	unreadable []string
}

// scanTrees walks every start folder and returns the files it found. It returns ctx's error when the context is
// cancelled, and no records, so a cancelled scan can never be committed by mistake. Every goroutine it starts
// has returned before it returns.
func scanTrees(ctx context.Context, fsys platform.FS, cfg scanConfig, starts []scanStart, counters *scanCounters) (scanResult, error) {
	if len(starts) == 0 {
		return scanResult{}, ctx.Err()
	}
	s := &scanner{fsys: fsys, cfg: cfg, queue: newWorkQueue(), counters: counters}
	for _, start := range starts {
		s.queue.push(start)
	}
	stop := context.AfterFunc(ctx, s.queue.close)
	defer stop()
	workers := cfg.workers
	if workers <= 0 {
		workers = scanWorkers
	}
	group := safego.NewGroup(safego.WithContext(ctx))
	for range workers {
		group.Go("files.scan.worker", func(context.Context) error {
			s.work()
			return nil
		})
	}
	_ = group.Wait()
	if err := ctx.Err(); err != nil {
		return scanResult{}, err
	}
	slices.Sort(s.unreadable)
	return scanResult{records: s.records, unreadable: s.unreadable, truncated: s.halted.Load()}, nil
}

// work is one worker: it takes folders until the queue closes, then merges what it collected.
func (s *scanner) work() {
	var local []record
	defer s.merge(&local)
	for {
		item, ok := s.queue.pop()
		if !ok {
			return
		}
		s.run(item, &local)
	}
}

// run reads one folder. The deferred done releases the queue slot even when reading panics, so the other
// workers can still finish; the panic itself is reported by safego.
func (s *scanner) run(item scanStart, out *[]record) {
	defer s.queue.done()
	s.process(item, out)
}

// merge appends a worker's records to the shared result.
func (s *scanner) merge(local *[]record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, *local...)
}

// process reads one folder and queues its subfolders, applying the hidden, ignore and symlink rules.
func (s *scanner) process(item scanStart, out *[]record) {
	entries, err := s.fsys.ReadDir(item.dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.addUnreadable(item.dir)
		}
		return
	}
	s.counters.dirs.Add(1)
	for _, de := range entries {
		name := de.Name()
		if !s.cfg.includeHidden && strings.HasPrefix(name, ".") {
			continue
		}
		isDir := de.IsDir()
		if s.cfg.rules.Ignored(item.rel, name, isDir) {
			continue
		}
		mode := de.Type()
		switch {
		case mode&fs.ModeSymlink != 0:
			// A link is never followed and never indexed, so a link loop cannot make the walk unbounded.
		case isDir:
			if item.remaining > 0 {
				s.queue.push(scanStart{dir: filepath.Join(item.dir, name), rel: joinRel(item.rel, name), remaining: item.remaining - 1})
			}
		case mode.IsRegular():
			if !s.addFile(item.dir, de, out) {
				return
			}
		}
	}
}

// addFile adds one file, reading its metadata when the sort needs it. It reports false once the entry cap is
// reached, which also closes the queue so the walk stops.
func (s *scanner) addFile(dir string, de fs.DirEntry, out *[]record) bool {
	if s.halted.Load() {
		return false
	}
	rec := record{dir: dir, name: de.Name()}
	if s.cfg.statFiles {
		info, err := de.Info()
		if err != nil {
			return true
		}
		rec.size = info.Size()
		rec.modMs = timeToMillis(info.ModTime())
		rec.createdMs = rec.modMs
	}
	if s.total.Add(1) > int64(s.cfg.maxEntries) {
		s.halted.Store(true)
		s.queue.close()
		return false
	}
	s.counters.files.Add(1)
	*out = append(*out, rec)
	return true
}

// addUnreadable records a folder that could not be read for a reason other than its disappearance.
func (s *scanner) addUnreadable(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unreadable = append(s.unreadable, dir)
}

// joinRel appends one name to a relative folder path.
func joinRel(rel, name string) string {
	if rel == "" {
		return name
	}
	return rel + "/" + name
}

// timeToMillis converts a time to Unix milliseconds, mapping the zero time to zero.
func timeToMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
