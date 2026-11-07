package extview

import "github.com/pablopunk/nvm/native/internal/ext/api"

// invokeRefresh marks host-owned refreshes so the command can revalidate its snapshot.
func (s *Session) invokeRefresh(ref api.ActionRef, done func(api.Result, *api.Failure)) {
	if s.dismissed {
		return
	}
	in := api.Invocation{Ref: ref, ProtectSnapshot: s.snapshotRef != nil, Launch: api.LaunchContext{Refresh: true, Reason: "refresh"}}
	if s.snapshotRef != nil {
		var version uint64
		paths := s.snapshotSourcePaths
		if s.deps.CommandSnapshots != nil {
			version = s.deps.CommandSnapshots.Version(s.snapshotIdentity, *s.snapshotRef)
			if paths == nil {
				paths = s.deps.CommandSnapshots.SnapshotPaths(s.snapshotIdentity, *s.snapshotRef)
			}
		}
		in.OnSnapshotDenied = func(denial *api.SnapshotDenial) {
			s.onUI(func() {
				if !snapshotPathsCover(paths, denial) {
					return
				}
				if s.deps.CommandSnapshots != nil {
					s.deps.CommandSnapshots.RemoveVersion(s.snapshotIdentity, *s.snapshotRef, version)
				}
				if s.dismissed || (s.deps.CommandSnapshotIdentity != nil && !sameSnapshotIdentity(s.snapshotIdentity, s.deps.CommandSnapshotIdentity(s.extID))) {
					return
				}
				s.clearPrivateSnapshot(denial.Failure)
			})
		}
	}
	if s.deps.InvokeRefresh != nil {
		s.deps.InvokeRefresh(s.workContext, s.extID, in, done)
		return
	}
	if s.deps.Invoke == nil {
		done(api.Result{}, &api.Failure{Code: api.CodeUnavailable, Message: unavailableMessage})
		return
	}
	s.deps.Invoke(s.extID, in, done)
}
