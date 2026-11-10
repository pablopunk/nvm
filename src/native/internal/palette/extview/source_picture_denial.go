package extview

import (
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"time"
)

// A late read may filter only the source snapshot from which this session obtained its references.
func (s *Session) filterSourcePictureDenial(denial *api.SnapshotDenial) {
	key := s.snapshotKey()
	if key == "" || s.sourcePictureVersion == nil {
		return
	}
	entry, ok := s.deps.Cache.Get(key)
	if !ok || entry.PictureVersion != s.sourcePictureVersion {
		return
	}
	sameIdentity := sameSnapshotIdentity(entry.PictureIdentity, s.installationIdentity) || (entry.PictureIdentity == nil && s.installationIdentity == nil)
	if !sameIdentity {
		return
	}
	if denial.AuthorityDenied {
		entry.Items = nil
		entry.PictureEvidence = nil
		entry.At = time.Time{}
	} else {
		// Data-source snapshots include action panels, unlike inert command snapshots: copy before filtering them.
		data, err := json.Marshal(entry.Items)
		if err != nil {
			return
		}
		var items []api.Item
		if json.Unmarshal(data, &items) != nil {
			return
		}
		view := api.View{Items: items}
		result := api.Result{View: &view, SnapshotEvidence: &api.SnapshotEvidence{DeniedPaths: []string{denial.Path}}}
		filterSnapshotDenied(&result, nil)
		entry.Items = view.Items
		entry.PictureEvidence = withoutPictureDenial(entry.PictureEvidence, denial)
	}
	entry.PictureVersion = new(byte)
	s.sourcePictureVersion = entry.PictureVersion
	s.deps.Cache.Put(key, entry)
}
