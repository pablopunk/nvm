package extview

import "github.com/pablopunk/nvm/native/internal/ext/api"

func (s *Session) revalidateMutation(extID string, identity any) {
	if identity == nil && extID == s.extID {
		identity = s.installationIdentity
	}
	if s.deps.Cache != nil {
		s.deps.Cache.invalidate(extID, identity)
	}

	for origin := s; origin != nil; origin = origin.parent {
		if !origin.currentInstallation() {
			continue
		}
		if origin.extID == extID && origin.view.Source != nil && (origin.view.Source.Loader != "" || origin.view.Refresh == nil) && !origin.dismissed && (identity == nil && origin.installationIdentity == nil || sameSnapshotIdentity(identity, origin.installationIdentity)) {
			origin.reload()
		} else {
			origin.refreshAfterMutation()
		}
	}
}

// refreshAfterMutation coalesces mutations behind an in-flight refresh.
func (s *Session) refreshAfterMutation() {
	if s.dismissed {
		return
	}
	if s.refreshing {
		s.refreshPending = true
		return
	}
	s.refreshNow()
}

// currentInstallation rejects work belonging to a replaced installation.
func (s *Session) currentInstallation() bool {
	if s.deps.CommandSnapshotIdentity == nil || s.installationIdentity == nil {
		return true
	}
	return sameSnapshotIdentity(s.installationIdentity, s.deps.CommandSnapshotIdentity(s.extID))
}

// cacheRefreshPatch only writes results from the generation that started the refresh.
func (s *Session) cacheRefreshPatch(patch api.ViewPatch, generation uint64) {
	key := s.snapshotKey()
	if key == "" {
		return
	}
	if patch.Mode == api.PatchReplace {
		s.sourceLease.put(key, generation, CacheEntry{Items: s.items, At: s.deps.Clock.Now()})
		return
	}
	if len(patch.Items) == 0 && len(patch.RemoveItemIDs) == 0 {
		return
	}
	if entry, _, ok := s.sourceLease.get(key); ok {
		entry.Items = applyPatch(entry.Items, patch)
		s.sourceLease.put(key, generation, entry)
	}
}

// sourceInvalidated runs on the UI thread for the visible matching installation.
func (s *Session) sourceInvalidated(identity any) {
	if s.dismissed || !s.currentInstallation() {
		return
	}
	if identity != nil || s.installationIdentity != nil {
		if !sameSnapshotIdentity(identity, s.installationIdentity) {
			return
		}
	}
	s.reload()
}

// revealSource revalidates covered rows if their installation epoch changed.
func (s *Session) revealSource() {
	if s.view.Source != nil && s.sourceLease != nil && s.sourceEpoch != s.sourceLease.version() {
		s.reload()
	}
	if s.view.Refresh != nil {
		s.refreshNow()
	}
}

// bindViewInvocation carries the originating session's installation through asynchronous dispatch.
func (s *Session) bindViewInvocation(extID string, in api.Invocation) api.Invocation {
	in.RequireIdentity = true
	if extID == s.extID {
		in.ExpectedIdentity = s.installationIdentity
	}
	return in
}
