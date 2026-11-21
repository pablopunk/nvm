package builtin

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"sync"
)

// observedCall isolates concurrent built-in invocations; late services cannot change a returned scope's evidence.
func (r *Runner) observedCall() (Runner, func() *api.SnapshotEvidence) {
	var mu sync.Mutex
	evidence := &api.SnapshotEvidence{}
	accepting := true
	call := *r
	call.context = r.context.Observed(func(request protocol.ServiceCall, reply protocol.ServiceReply) {
		mu.Lock()
		defer mu.Unlock()
		if accepting {
			supervisor.ObserveSnapshotService(evidence, request, reply)
		}
	})
	return call, func() *api.SnapshotEvidence {
		mu.Lock()
		defer mu.Unlock()
		accepting = false
		return evidence
	}
}
