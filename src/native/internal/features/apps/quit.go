package apps

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Quit timing: Quit asks an app to close with SIGTERM, waits QuitGrace on the Clock, and sends SIGKILL to what is
// still running. Force Quit sends SIGKILL at once, after the user confirms.
const (
	DefaultQuitGrace    = 5 * time.Second
	defaultQuitInterval = 250 * time.Millisecond
	// kernelCommLength is the longest process name the process table reports.
	kernelCommLength = 15
)

// QuitExceptions are processes "Quit All Apps" never asks to close: the desktop shell and the file manager's
// desktop. Nevermind itself is excluded by process ID, not by name.
var QuitExceptions = []string{"gnome-shell", "plasmashell", "xfce4-panel", "xfdesktop", "nautilus-desktop", "Finder"}

// QuitOutcome says what a quit request did to the processes it found.
type QuitOutcome struct {
	// Found is how many processes matched the app when the request started.
	Found int
	// Terminated is how many processes exited after SIGTERM, without SIGKILL.
	Terminated int
	// Killed is how many processes received SIGKILL, at once for Force Quit or after the grace period for Quit.
	Killed int
	// Failed counts signals the OS refused, other than a process that had already exited.
	Failed int
}

// Quitter sends signals to an app's processes through the Processes port. It signals single process IDs and never
// process groups, so a helper process of an app is signalled only when it runs under the app's own name, and the
// group of Nevermind is never signalled. It never signals process 0, process 1 or itself.
type Quitter struct {
	Processes platform.Processes
	Clock     platform.Clock
	// Self is Nevermind's process ID, which is never signalled.
	Self int
	// Grace is how long Quit waits after SIGTERM. Zero means DefaultQuitGrace.
	Grace time.Duration
	// Poll is how often Quit checks for exited processes. Zero means defaultQuitInterval.
	Poll time.Duration
}

// NewQuitter returns a quitter for the current process.
func NewQuitter(processes platform.Processes, clock platform.Clock) *Quitter {
	return &Quitter{Processes: processes, Clock: clock, Self: os.Getpid()}
}

// Quit asks the processes named by names to exit, escalating to SIGKILL for the ones still running after Grace.
func (q *Quitter) Quit(names []string) (QuitOutcome, error) {
	return q.signal(names, false)
}

// ForceQuit sends SIGKILL to the processes named by names at once. Callers confirm with the user first.
func (q *Quitter) ForceQuit(names []string) (QuitOutcome, error) {
	return q.signal(names, true)
}

func (q *Quitter) signal(names []string, force bool) (QuitOutcome, error) {
	if q.Processes == nil {
		return QuitOutcome{}, platform.ErrUnsupported
	}
	first, err := q.matching(names)
	if err != nil {
		return QuitOutcome{}, err
	}
	out := QuitOutcome{Found: len(first)}
	if len(first) == 0 {
		return out, nil
	}
	if force {
		q.send(first, platform.SignalKill, &out)
		return out, nil
	}
	q.send(first, platform.SignalTerminate, &out)
	grace := q.Grace
	if grace <= 0 {
		grace = DefaultQuitGrace
	}
	poll := q.Poll
	if poll <= 0 {
		poll = defaultQuitInterval
	}
	var waited time.Duration
	for {
		remaining, err := q.matching(names)
		if err != nil {
			return out, err
		}
		if len(remaining) == 0 {
			out.Terminated = len(first) - out.Failed
			return out, nil
		}
		if waited >= grace {
			// Re-list before each kill: a PID can exit and be reused, so only the processes still matching are killed.
			q.send(remaining, platform.SignalKill, &out)
			out.Terminated = len(first) - len(remaining) - out.Failed
			return out, nil
		}
		<-q.Clock.After(poll)
		waited += poll
	}
}

// send signals each process and records the outcome. A process that already exited is not a failure.
func (q *Quitter) send(pids []int, sig platform.Signal, out *QuitOutcome) {
	for _, pid := range pids {
		err := q.Processes.Signal(pid, sig)
		switch {
		case err == nil:
			if sig == platform.SignalKill {
				out.Killed++
			}
		case errors.Is(err, platform.ErrNotFound):
		default:
			out.Failed++
		}
	}
}

// matching returns the process IDs whose name is one of names, excluding process 0, process 1 and this process.
func (q *Quitter) matching(names []string) ([]int, error) {
	list, err := q.Processes.List()
	if err != nil {
		return nil, err
	}
	return SignalablePIDs(list, names, q.Self), nil
}

// SignalablePIDs picks the processes a quit may signal: those named by names (matched case-insensitively on the
// kernel's truncated name too), excluding process 0, process 1 and self.
func SignalablePIDs(list []platform.Process, names []string, self int) []int {
	wanted := map[string]bool{}
	for _, name := range names {
		lower := strings.ToLower(strings.TrimSpace(name))
		if lower == "" {
			continue
		}
		wanted[lower] = true
		if len(lower) > kernelCommLength {
			wanted[lower[:kernelCommLength]] = true
		}
	}
	var pids []int
	for _, process := range list {
		if process.PID <= 1 || process.PID == self {
			continue
		}
		if wanted[strings.ToLower(process.Name)] {
			pids = append(pids, process.PID)
		}
	}
	return pids
}

// IsExcludedFromQuitAll reports whether a process name is on the quit-all exception list.
func IsExcludedFromQuitAll(name string) bool {
	for _, exception := range QuitExceptions {
		if strings.EqualFold(name, exception) {
			return true
		}
	}
	return false
}
