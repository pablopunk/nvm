package menubar

import (
	"context"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Render is one unit of status item work. It runs off the UI thread under its budget and returns the change to apply
// on the UI thread, or nil when there is nothing to show.
type Render func(ctx context.Context) (apply func())

// Job describes when a named render runs. Every is the cadence (zero runs only when asked with Scheduler.Now). First
// is the delay before the first run after Schedule, and Budget bounds one run.
type Job struct {
	Every  time.Duration
	First  time.Duration
	Budget time.Duration
	Run    Render
}

// job is a scheduled Job with its next due time and whether a run is in flight.
type job struct {
	spec Job
	next time.Time
	busy bool
}

// Scheduler runs status item renders on their own cadence. Renders never run on the UI thread and never overlap
// themselves; their results return through OnUI. Time comes from the clock, so tests advance it instead of sleeping.
// The app's timer title and extension menu bar items are both jobs here.
type Scheduler struct {
	clock platform.Clock
	onUI  func(fn func())
	wake  chan struct{}
	mu    sync.Mutex
	jobs  map[string]*job
	runs  sync.WaitGroup
}

// NewScheduler returns a scheduler with no jobs. Run drives it.
func NewScheduler(clock platform.Clock, onUI func(fn func())) *Scheduler {
	return &Scheduler{clock: clock, onUI: onUI, wake: make(chan struct{}, 1), jobs: map[string]*job{}}
}

// Schedule creates or replaces the job named name. Its first run is due First after now; a job with neither a
// cadence nor a delay runs only when asked with Now. Replacing a job keeps a run already in flight.
func (s *Scheduler) Schedule(name string, spec Job) {
	s.mu.Lock()
	current, ok := s.jobs[name]
	if !ok {
		current = &job{}
		s.jobs[name] = current
	}
	current.spec = spec
	current.next = s.firstDue(spec)
	s.mu.Unlock()
	s.signal()
}

// firstDue is the next run time a job gets when it is scheduled. The caller holds s.mu.
func (s *Scheduler) firstDue(spec Job) time.Time {
	if spec.Every == 0 && spec.First == 0 {
		return time.Time{}
	}
	return s.clock.Now().Add(spec.First)
}

// Now makes the job named name due at once. An unknown name does nothing.
func (s *Scheduler) Now(name string) {
	s.mu.Lock()
	if current, ok := s.jobs[name]; ok {
		current.next = s.clock.Now()
	}
	s.mu.Unlock()
	s.signal()
}

// Cancel removes the job named name. A run already in flight still finishes and applies its result.
func (s *Scheduler) Cancel(name string) {
	s.mu.Lock()
	delete(s.jobs, name)
	s.mu.Unlock()
}

// Tick starts every job that is due and not running. It returns at once: the runs are in goroutines.
func (s *Scheduler) Tick() {
	now := s.clock.Now()
	s.mu.Lock()
	var due []string
	for name, current := range s.jobs {
		if !current.busy && !current.next.IsZero() && !now.Before(current.next) {
			due = append(due, name)
		}
	}
	for _, name := range due {
		s.start(s.jobs[name], now)
	}
	s.mu.Unlock()
}

// start marks one job running. The caller holds s.mu.
func (s *Scheduler) start(current *job, now time.Time) {
	current.busy = true
	if current.spec.Every > 0 {
		current.next = now.Add(current.spec.Every)
	} else {
		current.next = time.Time{}
	}
	s.runs.Add(1)
	go s.execute(current, current.spec)
}

// execute runs one render under its budget and hands the apply function to the UI thread.
func (s *Scheduler) execute(current *job, spec Job) {
	defer s.runs.Done()
	ctx, cancel := context.WithTimeout(context.Background(), budgetOf(spec))
	defer cancel()
	var apply func()
	if spec.Run != nil {
		apply = spec.Run(ctx)
	}
	s.mu.Lock()
	current.busy = false
	s.mu.Unlock()
	if apply != nil {
		s.onUI(apply)
	}
	s.signal()
}

// Run ticks the scheduler whenever the next job is due, or a job was added or woken, until ctx ends.
func (s *Scheduler) Run(ctx context.Context) {
	for {
		s.Tick()
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-s.clock.After(s.untilNext()):
		}
	}
}

// Wait blocks until every run started so far has finished. Tests use it after Tick; the app never waits.
func (s *Scheduler) Wait() {
	s.runs.Wait()
}

// untilNext is the time until the earliest idle job is due. It is a long wait when nothing is scheduled.
func (s *Scheduler) untilNext() time.Duration {
	const idle = time.Hour
	now := s.clock.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	wait := idle
	for _, current := range s.jobs {
		if current.busy || current.next.IsZero() {
			continue
		}
		if d := current.next.Sub(now); d < wait {
			wait = max(d, 0)
		}
	}
	return wait
}

// signal wakes Run without blocking.
func (s *Scheduler) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// budgetOf is the run's deadline. A job without a budget gets a generous one, so a stuck render cannot hold its slot.
func budgetOf(spec Job) time.Duration {
	if spec.Budget > 0 {
		return spec.Budget
	}
	return 5 * time.Second
}
