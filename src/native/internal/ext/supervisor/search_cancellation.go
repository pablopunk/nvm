package supervisor

import "github.com/pablopunk/nvm/native/internal/ext/protocol"

// cancelRequestChild retires an interrupted search without recording a failure. Dormancy preserves the
// declared methods so the manager can admit the next search while its replacement starts.
func (r *Runtime) cancelRequestChild(child *childProcess, method string, err error) error {
	if method != protocol.MethodSearch {
		return r.cancelViewChild(child, err)
	}
	r.mu.Lock()
	if r.child == child {
		r.declared = append([]string(nil), child.methods...)
		r.detachChildLocked()
		_ = child.proc.Kill()
		r.state = StateDormant
		r.restartingSearch = true
		r.wakeLocked()
	}
	r.mu.Unlock()
	return err
}
