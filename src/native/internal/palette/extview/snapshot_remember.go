package extview

import "github.com/pablopunk/nvm/native/internal/ext/api"

// rememberVerifiedSnapshot keeps a refreshed grid for the next open while the view still belongs to the installation that
// produced it.
func (s *Session) rememberVerifiedSnapshot(evidence *api.SnapshotEvidence) {
	if s.snapshotRef == nil || s.deps.CommandSnapshots == nil || s.deps.CommandSnapshotIdentity == nil || !sameSnapshotIdentity(s.snapshotIdentity, s.deps.CommandSnapshotIdentity(s.extID)) {
		return
	}
	s.deps.CommandSnapshots.PutVerified(s.snapshotIdentity, *s.snapshotRef, s.view, s.items, evidence)
}
