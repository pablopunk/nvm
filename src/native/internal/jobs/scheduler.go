package jobs

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/store"
)

// Defaults for the wake detector: a heartbeat every 10 seconds, and a gap of 30 seconds between two beats means the
// machine slept.
const (
	DefaultHeartbeat = 10 * time.Second
	DefaultSleepGap  = 30 * time.Second
)

// Config wires the scheduler to its ports. Clock is required. A nil State keeps the job state in memory only.
type Config struct {
	Clock platform.Clock
	// Reporter receives every failure of a run and of the scheduler's own goroutines. Nil uses safego's default.
	Reporter safego.Reporter
	// RunObserver observes actual job executions. Nil keeps the scheduler's legacy behavior.
	RunObserver RunObserver
	// State persists the job state; see StateDocument.
	State *store.Collection[State]
	// Log receives warnings about halted jobs and failed saves. Nil uses logx.Default.
	Log *logx.Logger
	// Heartbeat and SleepGap tune wake detection; zero means the defaults above.
	Heartbeat time.Duration
	SleepGap  time.Duration
}

// Scheduler owns every job of the process. It is safe for concurrent use. Runs never hold its lock, so a slow job
// never delays a Snapshot, a control or another job.
type Scheduler struct {
	clock       platform.Clock
	reporter    safego.Reporter
	runObserver RunObserver
	doc         *store.Collection[State]
	log         *logx.Logger
	heartbeat   time.Duration
	sleepGap    time.Duration
	ctx         context.Context
	cancel      context.CancelFunc

	mu      sync.Mutex
	closed  bool
	jobs    map[string]*record
	state   State
	subs    map[int]chan struct{}
	nextSub int
}

// New starts a scheduler on cfg. Triggers start when jobs are registered, and wake detection starts now.
func New(cfg Config) (*Scheduler, error) {
	if cfg.Clock == nil {
		return nil, errors.New("jobs: a scheduler needs a clock")
	}
	ctx, cancel := context.WithCancel(context.Background())
	heartbeat := cfg.Clock.NewTicker(positiveOr(cfg.Heartbeat, DefaultHeartbeat))
	last := cfg.Clock.Now()
	s := &Scheduler{
		clock:       cfg.Clock,
		reporter:    cfg.Reporter,
		runObserver: cfg.RunObserver,
		doc:         cfg.State,
		log:         cfg.Log,
		heartbeat:   positiveOr(cfg.Heartbeat, DefaultHeartbeat),
		sleepGap:    positiveOr(cfg.SleepGap, DefaultSleepGap),
		ctx:         ctx,
		cancel:      cancel,
		jobs:        map[string]*record{},
		state:       loadState(cfg.State),
		subs:        map[int]chan struct{}{},
	}
	if s.log == nil {
		s.log = logx.Default()
	}
	s.goGuarded("jobs.wake", func() {
		s.watchWake(heartbeat, last)
	})
	return s, nil
}

func positiveOr(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

// Register adds a job or replaces the definition of one with the same ID. The job keeps its saved state: whether the
// user enabled it, its history and any halt. Its triggers are installed when it is enabled.
func (s *Scheduler) Register(def Definition) error {
	if err := def.validate(); err != nil {
		return err
	}
	if def.Owner == "" {
		def.Owner = OwnerHost
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	r, ok := s.jobs[def.ID]
	if ok {
		r.def = def
	} else {
		r = newRecord(def, s.state.Jobs[def.ID])
		s.jobs[def.ID] = r
	}
	s.installLocked(r)
	s.notifyLocked()
	return nil
}

// Unregister removes a job and stops its triggers. A run already in progress finishes, and its result is dropped.
func (s *Scheduler) Unregister(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.jobs[id]
	if !ok {
		return
	}
	s.stopLocked(r)
	delete(s.jobs, id)
	s.notifyLocked()
}

// Has reports whether a job with id is registered.
func (s *Scheduler) Has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.jobs[id]
	return ok
}

// Snapshot copies every registered job, ordered by title.
func (s *Scheduler) Snapshot() []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	snapshots := make([]Snapshot, 0, len(s.jobs))
	for _, r := range s.jobs {
		snapshots = append(snapshots, r.snapshot(now))
	}
	slices.SortFunc(snapshots, func(a, b Snapshot) int {
		if a.Title != b.Title {
			return strings.Compare(a.Title, b.Title)
		}
		return strings.Compare(a.ID, b.ID)
	})
	return snapshots
}

// Subscribe returns a channel that receives after a job changed state, and a function that stops the subscription.
// Notifications coalesce: a receive means something changed since the previous receive.
func (s *Scheduler) Subscribe() (<-chan struct{}, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextSub
	s.nextSub++
	ch := make(chan struct{}, 1)
	s.subs[id] = ch
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.subs, id)
	}
}

// Close stops every trigger and the wake detector, cancels the context of runs in progress and flushes the saved
// state. Calls after Close return ErrClosed or do nothing.
func (s *Scheduler) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	for _, r := range s.jobs {
		s.stopLocked(r)
	}
	s.mu.Unlock()
	s.cancel()
	if s.doc != nil {
		return s.doc.Flush()
	}
	return nil
}

// notifyLocked tells subscribers that state changed. The caller holds s.mu; sends never block.
func (s *Scheduler) notifyLocked() {
	for _, ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// saveLocked writes one job's persisted state to memory and, when there is a document, to the store.
func (s *Scheduler) saveLocked(r *record) {
	id := r.def.ID
	saved := r.persisted()
	s.state.Jobs[id] = saved
	if s.doc == nil {
		return
	}
	if err := s.doc.Update(func(state *State) {
		if state.Jobs == nil {
			state.Jobs = map[string]JobState{}
		}
		state.Jobs[id] = saved
	}); err != nil {
		s.log.Warn("jobs.save.failed", logx.F{"job": id, "error": err.Error()})
	}
}

// goGuarded runs fn on its own goroutine under safego, so a panic in scheduler code is reported and not fatal.
func (s *Scheduler) goGuarded(name string, fn func()) {
	safego.Go(name, func(context.Context) { fn() }, s.safegoOptions()...)
}

// safegoOptions are the safego options of every call the scheduler makes. A reporter is passed only when the
// scheduler has one, so the process-wide default reporter stays in effect otherwise.
func (s *Scheduler) safegoOptions(extra ...safego.Option) []safego.Option {
	opts := append([]safego.Option{safego.WithContext(s.ctx)}, extra...)
	if s.reporter != nil {
		opts = append(opts, safego.WithReporter(s.reporter))
	}
	return opts
}

// once returns a one-shot timer for d, or nil when d is not positive, which means the wait is immediate.
func (s *Scheduler) once(d time.Duration) platform.Ticker {
	if d <= 0 {
		return nil
	}
	return s.clock.NewTicker(d)
}

// await waits for timer (nil means now) and reports true, or false when stop closes or the scheduler closes first.
// It stops the timer when it returns.
func (s *Scheduler) await(timer platform.Ticker, stop <-chan struct{}) bool {
	if timer == nil {
		select {
		case <-stop:
			return false
		case <-s.ctx.Done():
			return false
		default:
			return true
		}
	}
	defer timer.Stop()
	select {
	case <-timer.C():
		return true
	case <-stop:
		return false
	case <-s.ctx.Done():
		return false
	}
}
