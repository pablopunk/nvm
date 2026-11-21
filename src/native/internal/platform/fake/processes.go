package fake

import (
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Operation names accepted by Processes.FailNext.
const (
	OpProcessesList   = "List"
	OpProcessesSignal = "Signal"
)

// SignalCall is one recorded Signal call.
type SignalCall struct {
	PID    int
	Signal platform.Signal
}

// Processes is a platform.Processes over a fixed table. A terminate or kill signal removes the process from
// the table, as a process that exits would; interrupt leaves it running. Signalling a missing process returns
// ErrNotFound and is still recorded.
//
//	procs := fake.NewProcesses(platform.Process{PID: 42, Name: "editor", ResidentKB: 90000})
//	procs.Signal(42, platform.SignalTerminate) // the process is gone from List afterwards
type Processes struct {
	mu      sync.Mutex
	table   map[int]platform.Process
	signals []SignalCall
	faults  faultSet
}

var _ platform.Processes = (*Processes)(nil)

// NewProcesses returns a fake holding the given processes.
func NewProcesses(processes ...platform.Process) *Processes {
	table := make(map[int]platform.Process, len(processes))
	for _, process := range processes {
		table[process.PID] = process
	}
	return &Processes{table: table}
}

// FailNext makes the next call of op return err. Use the Op constants of this file.
func (p *Processes) FailNext(op string, err error) {
	p.faults.failNext(op, err)
}

// List returns the processes sorted by PID.
func (p *Processes) List() ([]platform.Process, error) {
	if err := p.faults.take(OpProcessesList); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	list := make([]platform.Process, 0, len(p.table))
	for _, process := range p.table {
		list = append(list, process)
	}
	slices.SortFunc(list, func(a, b platform.Process) int { return a.PID - b.PID })
	return list, nil
}

// Signal records the request and applies it to the table.
func (p *Processes) Signal(pid int, sig platform.Signal) error {
	if err := p.faults.take(OpProcessesSignal); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.signals = append(p.signals, SignalCall{PID: pid, Signal: sig})
	if _, ok := p.table[pid]; !ok {
		return platform.ErrNotFound
	}
	if sig == platform.SignalTerminate || sig == platform.SignalKill {
		delete(p.table, pid)
	}
	return nil
}

// Signals returns the recorded signal requests in order.
func (p *Processes) Signals() []SignalCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.signals)
}
