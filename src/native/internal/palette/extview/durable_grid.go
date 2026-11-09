package extview

import (
	"context"
	"crypto/sha256"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// revalidateDurableGrid reconstructs authority for inert persisted rows before any of their content is displayed.
// A missing, changed, unreadable or invalid file is omitted; cancellation discards the entire result.
func revalidateDurableGrid(ctx context.Context, fsys platform.FS, view api.View) (api.View, *api.SnapshotEvidence, error) {
	evidence := &api.SnapshotEvidence{ThumbnailPaths: make(map[[32]byte][]string), ThumbnailHashes: make(map[[32]byte]bool)}
	items := view.Items
	view.Items = nil
	if fsys == nil || len(items) > 200 {
		return view, evidence, nil
	}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return api.View{}, nil, err
		}
		if item.Image == nil || !filepath.IsAbs(item.ID) {
			continue
		}
		path := filepath.Clean(item.ID)
		if item.Image.Src != durableGridReference(path) || thumbs.Probe(fsys, path) != nil {
			continue
		}
		hash := sha256.Sum256([]byte(item.Image.Src))
		evidence.ThumbnailHashes[hash] = true
		evidence.ThumbnailPaths[hash] = append(evidence.ThumbnailPaths[hash], path)
		evidence.ThumbnailSamples++
		view.Items = append(view.Items, item)
	}
	if err := ctx.Err(); err != nil {
		return api.View{}, nil, err
	}
	return view, evidence, nil
}

func durableGridReference(path string) string {
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return thumbs.GridReference((&url.URL{Scheme: "file", Path: slashed}).String())
}
