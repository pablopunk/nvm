package extview

import (
	"github.com/pablopunk/nvm/native/internal/search"
)

// runUpdate performs an update operation of the host's updater. A refused operation shows its reason as a toast.
func (s *Session) runUpdate(h search.Host, op string) {
	if err := s.deps.RunUpdate(op); err != nil {
		h.Toast(err.Error())
	}
}

// fileOperation runs a file operation the host owns and reports its failure as a toast. The palette stays open: Quick
// Look is a preview, not a hand-off to another app.
func (s *Session) fileOperation(h search.Host, run func(string) error, path string) {
	if run == nil {
		h.Toast(unavailableMessage)
		return
	}
	if err := run(path); err != nil {
		h.Toast(err.Error())
	}
}
