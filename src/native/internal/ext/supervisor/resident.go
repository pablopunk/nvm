package supervisor

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// watchResident samples the child's resident memory while it runs and kills it above ResidentLimit. The
// address-space cap only exists on Linux; this watch is the bound everywhere the OS can report process size.
// A platform that cannot report sizes records one diagnostic and stops watching.
func (r *Runtime) watchResident(child *childProcess) {
	if r.cfg.Processes == nil || r.cfg.ResidentLimit == 0 {
		return
	}
	ticker := r.cfg.Clock.NewTicker(memoryWatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-child.done:
			return
		case <-ticker.C():
			if r.checkResident(child) {
				return
			}
		}
	}
}

// checkResident reports true when the watch should stop: the child is gone, or the platform cannot say.
// It kills the child when the limit is exceeded.
func (r *Runtime) checkResident(child *childProcess) (stop bool) {
	resident, found, err := residentBytes(r.cfg.Processes, child.proc.PID())
	if err != nil {
		r.noteWatchUnavailable(err)
		return true
	}
	if !found {
		return true
	}
	if uint64(resident) <= r.cfg.ResidentLimit {
		return false
	}
	_ = r.failChild(child, api.CodeQuota, fmt.Sprintf("memory limit exceeded: %d MB resident", resident>>20))
	return true
}

func (r *Runtime) noteWatchUnavailable(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addDiagnosticLocked("resident memory watch unavailable: " + err.Error())
}

// residentBytes returns the resident size of the process with pid, and whether it was found.
func residentBytes(processes platform.Processes, pid int) (int64, bool, error) {
	list, err := processes.List()
	if err != nil {
		return 0, false, err
	}
	for _, process := range list {
		if process.PID == pid {
			return process.ResidentKB << 10, true, nil
		}
	}
	return 0, false, nil
}
