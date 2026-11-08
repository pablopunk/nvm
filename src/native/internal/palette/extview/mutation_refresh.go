package extview

import "github.com/pablopunk/nvm/native/internal/ext/api"

func (s *Session) revalidateMutation(extID string) {
	if s.deps.Cache != nil {
		identity := s.installationIdentity
		if extID != s.extID && s.deps.CommandSnapshotIdentity != nil {
			identity = s.deps.CommandSnapshotIdentity(extID)
		}
		s.deps.Cache.InvalidateExtension(s.deps.Cache.Scope(extID, identity))
	}
	for origin := s; origin != nil; origin = origin.parent {
		if !origin.currentInstallation() {
			continue
		}
		if origin.extID == extID && origin.view.Source != nil && !origin.dismissed {
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

// cacheScope binds snapshots to the installation that opened this session.
func (s *Session) cacheScope() string {
	return s.deps.Cache.Scope(s.extID, s.installationIdentity)
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
		s.deps.Cache.PutGeneration(s.cacheScope(), key, generation, CacheEntry{Items: s.items, At: s.deps.Clock.Now()})
		return
	}
	if len(patch.Items) == 0 && len(patch.RemoveItemIDs) == 0 {
		return
	}
	if entry, ok := s.deps.Cache.Get(key); ok {
		entry.Items = applyPatch(entry.Items, patch)
		s.deps.Cache.PutGeneration(s.cacheScope(), key, generation, entry)
	}
}
