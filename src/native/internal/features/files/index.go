package files

import (
	"context"
	"errors"
	"iter"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Defaults of the index configuration.
const (
	defaultMaxDepth   = 2
	defaultMaxEntries = 250_000
	defaultDebounce   = 150 * time.Millisecond
	defaultReconcile  = 10 * time.Minute
	defaultSaveEvery  = time.Minute
	eventBuffer       = 1024
)

// Config configures an Index. Only FS and Clock are required; the zero value of every other field picks the default
// described on it.
type Config struct {
	// Home is the user's home folder: the default roots live under it and display paths abbreviate it.
	Home string
	// Roots are the folders to index. Empty means DefaultRoots(Home). Relative roots are dropped.
	Roots []string
	// MaxDepth is how many folder levels below a root are read; zero means 2, as file-index.ts does.
	MaxDepth int
	// MaxEntries caps the number of files one scan keeps; zero means 250,000.
	MaxEntries int
	// IncludeHidden indexes names that start with a dot.
	IncludeHidden bool
	// Ignore adds patterns to the ignore rules (see IgnoreRules for the syntax).
	Ignore []string
	// SkipDefaultIgnores drops DefaultIgnores, so only Ignore applies.
	SkipDefaultIgnores bool
	// Workers is how many folders a scan reads at once; zero means 4.
	Workers int
	// Debounce is how long file system events are collected before they are applied; zero means 150 ms.
	Debounce time.Duration
	// Reconcile is how often the whole index is scanned again to repair events the OS dropped; zero means 10 minutes.
	Reconcile time.Duration
	// SaveEvery is the least time between two saves of the persisted snapshot; zero means one minute. A final save
	// happens on Close regardless.
	SaveEvery time.Duration
	// Persistence stores the snapshot between runs. Nil keeps the index in memory only.
	Persistence Persistence
	// OnChange runs in the index loop after every published snapshot. It must return quickly and must not call Close.
	OnChange func()
	// Logger receives warnings. Nil uses logx.Default().
	Logger *logx.Logger
	FS     platform.FS
	Clock  platform.Clock
}

// Status describes the index for a view. It is a copy taken at one moment.
type Status struct {
	// Ready is true once a snapshot is published, from the persisted copy or from a scan.
	Ready bool
	// FromPersistence is true while the published snapshot is the one loaded from disk and no scan has replaced it.
	FromPersistence bool
	Scanning        bool
	Entries         int
	Generation      uint64
	ScannedDirs     int64
	ScannedFiles    int64
	MissingRoots    []string
	UnreadableRoots []string
	Truncated       bool
	LastCommit      time.Time
	// SaveError is the message of the last failed save, or "" when the last save succeeded or none ran.
	SaveError string
}

// Index is the file name index. Its snapshot is replaced as a whole and read without locks; its state for Status is
// guarded by mu; everything that changes the snapshot runs in one loop goroutine.
type Index struct {
	cfg       Config
	roots     []string
	rules     IgnoreRules
	configKey string
	log       *logx.Logger

	snap     atomic.Pointer[Snapshot]
	counters scanCounters
	events   chan platform.Event
	lost     chan string
	trigger  chan struct{}
	done     chan struct{}
	cancel   context.CancelFunc
	started  atomic.Bool
	saving   atomic.Bool
	savedGen atomic.Uint64
	saves    sync.WaitGroup
	scanners sync.WaitGroup
	watchers sync.WaitGroup
	// lastSave is read and written by the loop goroutine only.
	lastSave time.Time

	mu     sync.Mutex
	status Status
}

// New validates cfg and returns an index that has not started. It reads no files; call Start to begin.
func New(cfg Config) (*Index, error) {
	if cfg.FS == nil || cfg.Clock == nil {
		return nil, errors.New("files: an index needs a file system and a clock")
	}
	roots := cfg.Roots
	if len(roots) == 0 {
		roots = DefaultRoots(cfg.Home)
	}
	roots = NormalizeRoots(cfg.Home, roots)
	if len(roots) == 0 {
		return nil, errors.New("files: an index needs at least one absolute root")
	}
	cfg = withDefaults(cfg)
	ix := &Index{
		cfg:     cfg,
		roots:   roots,
		rules:   CompileIgnoreRules(cfg.Ignore, !cfg.SkipDefaultIgnores),
		log:     cfg.Logger,
		events:  make(chan platform.Event, eventBuffer),
		lost:    make(chan string, 16),
		trigger: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	if ix.log == nil {
		ix.log = logx.Default()
	}
	ix.configKey = configKeyFor(roots, cfg, ix.rules)
	ix.snap.Store(emptySnapshot(ix.configKey))
	return ix, nil
}

// withDefaults fills the zero-valued options of cfg.
func withDefaults(cfg Config) Config {
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = defaultMaxDepth
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = defaultMaxEntries
	}
	if cfg.Workers <= 0 {
		cfg.Workers = scanWorkers
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = defaultDebounce
	}
	if cfg.Reconcile <= 0 {
		cfg.Reconcile = defaultReconcile
	}
	if cfg.SaveEvery <= 0 {
		cfg.SaveEvery = defaultSaveEvery
	}
	return cfg
}

// configKeyFor fingerprints everything that decides which files a snapshot holds. A persisted snapshot with another
// key is discarded rather than served.
func configKeyFor(roots []string, cfg Config, rules IgnoreRules) string {
	parts := []string{
		strings.Join(roots, "\x1f"),
		strconv.Itoa(cfg.MaxDepth),
		strconv.Itoa(cfg.MaxEntries),
		strconv.FormatBool(cfg.IncludeHidden),
		strconv.Itoa(len(rules.rules)),
	}
	for _, rule := range rules.rules {
		parts = append(parts, strconv.FormatBool(rule.negate), strconv.FormatBool(rule.dirOnly), rule.pattern)
	}
	return strings.Join(parts, "\x1e")
}

// Start begins loading the persisted snapshot and scanning the roots in the background. It returns at once. The
// index runs until ctx is cancelled or Close is called.
func (ix *Index) Start(ctx context.Context) error {
	if !ix.started.CompareAndSwap(false, true) {
		return errors.New("files: index already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	ix.cancel = cancel
	safego.Go("files.index", func(c context.Context) {
		defer close(ix.done)
		ix.run(c)
	}, safego.WithContext(runCtx))
	return nil
}

// Close stops the index, waits for its goroutines to return and saves the snapshot if it changed since the last save.
// It is safe to call more than once and before Start.
func (ix *Index) Close() error {
	if !ix.started.Load() {
		return nil
	}
	ix.cancel()
	<-ix.done
	ix.scanners.Wait()
	ix.watchers.Wait()
	ix.saves.Wait()
	return nil
}

// Snapshot returns the published snapshot. It never blocks and never returns nil.
func (ix *Index) Snapshot() *Snapshot { return ix.snap.Load() }

// Status returns a copy of the current status.
func (ix *Index) Status() Status {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	st := ix.status
	st.MissingRoots = append([]string(nil), st.MissingRoots...)
	st.UnreadableRoots = append([]string(nil), st.UnreadableRoots...)
	st.ScannedDirs = ix.counters.dirs.Load()
	st.ScannedFiles = ix.counters.files.Load()
	return st
}

// Refresh asks for a full scan as soon as the current one, if any, finishes. Calls while one is already queued are
// coalesced.
func (ix *Index) Refresh() {
	select {
	case ix.trigger <- struct{}{}:
	default:
	}
}

// Search streams the results of q over the snapshot published when the search starts. See Batch.
func (ix *Index) Search(ctx context.Context, q Query) iter.Seq[Batch] {
	return searchSnapshot(ctx, ix.Snapshot(), q, ix.env())
}

// env returns the search environment of this index.
func (ix *Index) env() searchEnv {
	return searchEnv{home: ix.cfg.Home, roots: ix.roots, skipDefaults: ix.cfg.SkipDefaultIgnores}
}

// scanConfig returns the walk settings of this index.
func (ix *Index) scanConfig(statFiles bool) scanConfig {
	return scanConfig{
		maxDepth:      ix.cfg.MaxDepth,
		maxEntries:    ix.cfg.MaxEntries,
		rules:         ix.rules,
		includeHidden: ix.cfg.IncludeHidden,
		statFiles:     statFiles,
		workers:       ix.cfg.Workers,
	}
}

// updateStatus mutates the status under the lock.
func (ix *Index) updateStatus(change func(*Status)) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	change(&ix.status)
}
