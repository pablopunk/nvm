package windowmgmt

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/logx"
)

// reportFailure writes a failed window command to the log with the cause the palette does not show. A command that
// reached its window, or a nil error, is not logged.
func (s *Service) reportFailure(op string, err error) {
	if err == nil {
		return
	}
	var failed *Failure
	if errors.As(err, &failed) {
		s.logger().Warn("window.command.failed", logx.F{"op": op, "reason": string(failed.Reason), "message": failed.Message, "cause": failed.Cause})
		return
	}
	s.logger().Warn("window.command.failed", logx.F{"op": op, "error": err.Error()})
}

// logger is where failed commands are written: Deps.Log, or the process logger when none was set.
func (s *Service) logger() *logx.Logger {
	if s.log != nil {
		return s.log
	}
	return logx.Default()
}
