package extview

import (
	"crypto/sha256"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"path/filepath"
)

// FilterPictureDenial changes only this installation and invocation's retained presentation.
func (c *CommandSnapshots) FilterPictureDenial(identity any, ref api.ActionRef, version uint64, denial *api.SnapshotDenial) {
	if denial == nil || version == 0 || c.Version(identity, ref) != version {
		return
	}
	if denial.AuthorityDenied {
		c.Remove(identity, ref)
		return
	}
	if !c.CoversDenial(identity, ref, denial) {
		return
	}
	view, ok := c.Get(identity, ref)
	if !ok {
		return
	}
	key, _ := snapshotKey(identity, ref)
	paths := make(map[[32]byte]bool, len(c.sourcePaths[key]))
	denied := sha256.Sum256([]byte(filepath.Clean(denial.Path)))
	for hash := range c.sourcePaths[key] {
		if hash != denied {
			paths[hash] = true
		}
	}
	result := api.Result{View: &view, SnapshotEvidence: &api.SnapshotEvidence{DeniedPaths: []string{denial.Path}}}
	filterSnapshotDenied(&result, c.staleHashes[key])
	c.Put(identity, ref, view, view.Items)
	if _, ok := c.entries[key]; ok {
		c.sourcePaths[key] = paths
	}
}
