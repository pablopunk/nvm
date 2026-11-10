package extview

import (
	"crypto/sha256"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"path/filepath"
)

func copyPictureEvidence(e *api.SnapshotEvidence) *api.SnapshotEvidence {
	if e == nil {
		return nil
	}
	copy := *e
	copy.ThumbnailHashes = make(map[[32]byte]bool, len(e.ThumbnailHashes))
	copy.ThumbnailPaths = make(map[[32]byte][]string, len(e.ThumbnailPaths))
	copy.DeniedPaths = append([]string(nil), e.DeniedPaths...)
	for hash, paths := range e.ThumbnailPaths {
		copy.ThumbnailPaths[hash] = append([]string(nil), paths...)
	}
	for hash, present := range e.ThumbnailHashes {
		copy.ThumbnailHashes[hash] = present
	}
	return &copy
}

func withoutPictureDenial(e *api.SnapshotEvidence, denial *api.SnapshotDenial) *api.SnapshotEvidence {
	copy := copyPictureEvidence(e)
	if copy == nil || denial == nil {
		return copy
	}
	if denial.AuthorityDenied {
		return nil
	}
	copy.DeniedPaths = append(copy.DeniedPaths, denial.Path)
	return copy
}

func (s *Session) applyPicturePatch(patch api.ViewPatch, evidence *api.SnapshotEvidence) {
	previous := s.keptPictureEvidence(patch)
	s.Apply(patch)
	next := copyPictureEvidence(evidence)
	if patch.Mode != api.PatchReplace && previous != nil {
		if next == nil {
			next = copyPictureEvidence(previous)
		} else if !next.FilesAuthorityDenied {
			for hash, paths := range previous.ThumbnailPaths {
				if _, exists := next.ThumbnailPaths[hash]; !exists {
					next.ThumbnailPaths[hash] = append([]string(nil), paths...)
					next.ThumbnailHashes[hash] = previous.ThumbnailHashes[hash]
				}
			}
			for _, denied := range previous.DeniedPaths {
				renewed := false
				for hash, paths := range next.ThumbnailPaths {
					if !next.ThumbnailHashes[hash] {
						continue
					}
					for _, path := range paths {
						if filepath.Clean(path) == filepath.Clean(denied) {
							renewed = true
						}
					}
				}
				if !renewed {
					next.DeniedPaths = append(next.DeniedPaths, denied)
				}
			}
		}
	}
	s.guard.Lock()
	s.pictureEvidence = next
	s.guard.Unlock()
}

// PictureEvidence rebinds only previously verified references when a retained presentation is opened in a new session.
func (c *CommandSnapshots) PictureEvidence(identity any, ref api.ActionRef) *api.SnapshotEvidence {
	view, ok := c.Get(identity, ref)
	if !ok {
		return nil
	}
	paths := c.SnapshotPaths(identity, ref)
	evidence := &api.SnapshotEvidence{ThumbnailHashes: make(map[[32]byte]bool), ThumbnailPaths: make(map[[32]byte][]string)}
	for _, item := range view.Items {
		if item.Image == nil || !paths[sha256.Sum256([]byte(filepath.Clean(item.ID)))] {
			continue
		}
		hash := sha256.Sum256([]byte(item.Image.Src))
		evidence.ThumbnailHashes[hash] = true
		evidence.ThumbnailPaths[hash] = []string{item.ID}
	}
	return evidence
}

// A patch may retain provenance only for rows it did not replace or remove.
func (s *Session) keptPictureEvidence(patch api.ViewPatch) *api.SnapshotEvidence {
	s.guard.Lock()
	previous := copyPictureEvidence(s.pictureEvidence)
	s.guard.Unlock()
	if previous == nil {
		return nil
	}
	changed := make(map[string]bool)
	for _, item := range patch.Items {
		changed[item.ID] = true
	}
	for _, id := range patch.RemoveItemIDs {
		changed[id] = true
	}
	retained := make(map[[32]byte]bool)
	collect := func(items []api.Item) {
		for _, item := range items {
			if changed[item.ID] || item.Image == nil {
				continue
			}
			for _, ref := range []string{item.Image.Src, item.Image.Light, item.Image.Dark, item.Image.Fallback} {
				retained[sha256.Sum256([]byte(ref))] = true
			}
		}
	}
	collect(s.items)
	for _, section := range s.sections {
		collect(section.Items)
	}
	for hash := range previous.ThumbnailHashes {
		if !retained[hash] {
			delete(previous.ThumbnailHashes, hash)
			delete(previous.ThumbnailPaths, hash)
		}
	}
	return previous
}
