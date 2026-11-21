package extview

import "github.com/pablopunk/nvm/native/internal/ext/api"

func (s *Session) putSnapshot(items []api.Item) {
	if key := s.snapshotKey(); key != "" {
		s.guard.Lock()
		s.sourcePictureVersion = new(byte)
		evidence := copyPictureEvidence(s.pictureEvidence)
		s.guard.Unlock()
		s.deps.Cache.Put(key, CacheEntry{Items: items, At: s.deps.Clock.Now(), PictureEvidence: evidence, PictureIdentity: s.installationIdentity, PictureLoader: s.view.Source.Loader, PictureVersion: s.sourcePictureVersion})
	}
}

// cacheRefreshPatch renews freshness only for a complete replacement. Partial patches update an existing snapshot
// without making its unchanged rows fresh, and never create a snapshot before the source has produced one.
func (s *Session) cacheRefreshPatch(patch api.ViewPatch) {
	if patch.Mode == api.PatchReplace {
		s.putSnapshot(s.items)
		return
	}
	if len(patch.Items) == 0 && len(patch.RemoveItemIDs) == 0 {
		return
	}
	if key := s.snapshotKey(); key != "" {
		if entry, ok := s.deps.Cache.Get(key); ok {
			entry.Items = applyPatch(entry.Items, patch)
			entry.PictureEvidence = copyPictureEvidence(s.pictureEvidence)
			entry.PictureIdentity = s.installationIdentity
			entry.PictureLoader = s.view.Source.Loader
			entry.PictureVersion = new(byte)
			s.sourcePictureVersion = entry.PictureVersion
			s.deps.Cache.Put(key, entry)
		}
	}
}

func (s *Session) installLoadedItems(items []api.Item, evidence *api.SnapshotEvidence) {
	s.resetGridImages()
	s.items = items
	s.guard.Lock()
	s.pictureEvidence = copyPictureEvidence(evidence)
	s.pictureInvocation = &api.ActionRef{ID: s.view.Source.Loader}
	s.guard.Unlock()
}

func (s *Session) restoreLoadedSnapshot(entry CacheEntry) {
	s.items = entry.Items
	sameIdentity := sameSnapshotIdentity(entry.PictureIdentity, s.installationIdentity) || (entry.PictureIdentity == nil && s.installationIdentity == nil)
	s.guard.Lock()
	if sameIdentity && entry.PictureLoader == s.view.Source.Loader {
		s.pictureEvidence = copyPictureEvidence(entry.PictureEvidence)
	} else {
		s.pictureEvidence = nil
	}
	s.pictureInvocation = &api.ActionRef{ID: s.view.Source.Loader}
	s.sourcePictureVersion = entry.PictureVersion
	s.guard.Unlock()
}

func (s *Session) pictureCommand() *api.ActionRef {
	if s.pictureInvocation != nil {
		return s.pictureInvocation
	}
	return s.snapshotRef
}
