package extview

import "github.com/pablopunk/nvm/native/internal/ext/api"

// snapshotDenialScope belongs to one invocation and presentation, rather than whichever view is current later.
type snapshotDenialScope struct {
	generation uint64
	version    uint64
}

func (s *Session) snapshotDenialHandler(paths map[[32]byte]bool, version uint64) func(*api.SnapshotDenial) {
	s.guard.Lock()
	scope := &snapshotDenialScope{generation: s.imageGeneration, version: version}
	s.snapshotDenials = scope
	s.guard.Unlock()
	identity := s.installationIdentity
	return func(denial *api.SnapshotDenial) {
		s.onUI(func() {
			if s.dismissed || !snapshotPathsCover(paths, denial) {
				return
			}
			if s.deps.CommandSnapshotIdentity != nil && !sameSnapshotIdentity(identity, s.deps.CommandSnapshotIdentity(s.extID)) {
				return
			}
			s.guard.Lock()
			current := s.snapshotDenials == scope && scope.generation == s.imageGeneration
			s.guard.Unlock()
			if !current {
				return
			}
			s.applySnapshotImageDenial(scope.generation, scope.version, denial)
			// A denial invalidates image work, but further denials from this same invocation are still applicable.
			s.guard.Lock()
			scope.generation = s.imageGeneration
			s.guard.Unlock()
		})
	}
}
