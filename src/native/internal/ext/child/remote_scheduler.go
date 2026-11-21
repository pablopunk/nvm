package child

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteScheduler schedules one-shot runs of the extension's own handlers in the host.
type remoteScheduler struct {
	link *hostLink
}

func (s remoteScheduler) Once(key string, at time.Time, ref api.ActionRef) error {
	return s.link.Call(protocol.ServiceScheduler, protocol.OpOnce, protocol.OnceArgs{Key: key, At: at, Ref: ref}, nil)
}

func (s remoteScheduler) Cancel(key string) (bool, error) {
	var cancelled bool
	err := s.link.Call(protocol.ServiceScheduler, protocol.OpCancel, protocol.ScheduleKeyArgs{Key: key}, &cancelled)
	return cancelled, err
}

func (s remoteScheduler) Pending(key string) (time.Time, bool, error) {
	var reply protocol.PendingReply
	err := s.link.Call(protocol.ServiceScheduler, protocol.OpPending, protocol.ScheduleKeyArgs{Key: key}, &reply)
	return reply.At, reply.Pending, err
}
