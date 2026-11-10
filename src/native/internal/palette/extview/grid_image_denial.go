package extview

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"github.com/pablopunk/nvm/native/internal/platform"
	"net/url"
	"path/filepath"
)

func (s *Session) resetGridImages() {
	s.itemsRevision++
	s.guard.Lock()
	s.snapshotDenials = nil
	s.pictureEvidence = nil
	s.guard.Unlock()
	s.advanceImages(func(ref string) bool { return thumbs.IsGridReference(ref) })
}

// advanceImages makes every older completion inert before changing images or action routes.
func (s *Session) advanceImages(remove func(string) bool) {
	s.guard.Lock()
	s.imageGeneration++
	s.gridScope = nil
	cancel := s.cancelImages
	s.imageContext, s.cancelImages = context.WithCancel(context.Background())
	for ref := range s.images {
		if remove(ref) || thumbs.IsGridReference(ref) {
			delete(s.images, ref)
			delete(s.imageAuthority, ref)
			delete(s.imageSnapshotVersions, ref)
		}
	}
	s.requested = make(map[string]bool, len(s.images))
	for ref := range s.images {
		s.requested[ref] = true
	}
	s.guard.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Session) applyGridDenial(ref string, generation uint64, version uint64, denial *api.SnapshotDenial) {
	if s.dismissed || denial == nil {
		return
	}
	s.guard.Lock()
	current := s.imageGeneration == generation
	s.guard.Unlock()
	if !current {
		return
	}
	parsed, err := url.Parse(ref)
	if err != nil || !thumbs.IsGridReference(ref) {
		return
	}
	path := filepath.Clean(platform.FileURLPath(parsed))
	if !denial.AuthorityDenied && (!filepath.IsAbs(denial.Path) || path != filepath.Clean(denial.Path)) {
		return
	}
	s.applySnapshotImageDenial(generation, version, denial)
}

// applySnapshotImageDenial also accepts path-scoped metadata denials from the supervisor.
func (s *Session) applySnapshotImageDenial(generation, version uint64, denial *api.SnapshotDenial) {
	if s.dismissed || denial == nil {
		return
	}
	s.guard.Lock()
	current := generation == s.imageGeneration
	s.guard.Unlock()
	if !current {
		return
	}
	s.rememberSnapshotImages()
	s.guard.Lock()
	s.pictureEvidence = withoutPictureDenial(s.pictureEvidence, denial)
	s.guard.Unlock()
	path := filepath.Clean(denial.Path)
	s.advanceImages(func(candidate string) bool {
		if denial.AuthorityDenied {
			return true
		}
		parsed, err := url.Parse(candidate)
		return err == nil && parsed.Scheme == "file" && filepath.Clean(platform.FileURLPath(parsed)) == path
	})
	evidence := &api.SnapshotEvidence{FilesAuthorityDenied: denial.AuthorityDenied}
	if !denial.AuthorityDenied {
		evidence.DeniedPaths = []string{path}
	}
	view := s.view
	view.Items = append([]api.Item(nil), s.items...)
	view.Sections = append([]api.ItemSection(nil), s.sections...)
	for i := range view.Sections {
		view.Sections[i].Items = append([]api.Item(nil), view.Sections[i].Items...)
	}
	result := api.Result{View: &view, SnapshotEvidence: evidence}
	filterSnapshotDenied(&result, s.snapshotStaleHashes)
	s.view, s.items, s.sections = view, view.Items, view.Sections
	s.guard.Lock()
	s.handles = make(map[handle]route)
	s.filePreviews = nil
	s.guard.Unlock()
	if s.snapshotRef != nil && s.deps.CommandSnapshots != nil {
		s.deps.CommandSnapshots.FilterPictureDenial(s.snapshotIdentity, *s.snapshotRef, version, denial)
	}
	if denial.AuthorityDenied {
		s.clearPrivateSnapshot(denial.Failure)
		return
	}
	if filepath.Clean(s.selectID) == path {
		s.selectID = ""
	}
	s.refreshHost()
}

func (s *Session) applyImageEvidenceDenial(version uint64, evidence *api.SnapshotEvidence, failure *api.Failure) {
	s.guard.Lock()
	generation := s.imageGeneration
	s.guard.Unlock()
	if evidence.FilesAuthorityDenied {
		s.applySnapshotImageDenial(generation, version, &api.SnapshotDenial{AuthorityDenied: true, Failure: failure})
		return
	}
	for _, path := range evidence.DeniedPaths {
		if !snapshotPathsCover(s.snapshotSourcePaths, &api.SnapshotDenial{Path: path}) {
			continue
		}
		s.applySnapshotImageDenial(generation, version, &api.SnapshotDenial{Path: path, Failure: failure})
		s.guard.Lock()
		generation = s.imageGeneration
		s.guard.Unlock()
	}
}
