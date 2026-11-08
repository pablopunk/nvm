package extview

import (
	"crypto/sha256"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
)

// PutVerified only retains thumbnails actually returned by the host files service in this run.
func (c *CommandSnapshots) PutVerified(identity any, ref api.ActionRef, view api.View, items []api.Item, evidence *api.SnapshotEvidence) {
	if evidence == nil || evidence.Overflow || evidence.FilesAuthorityDenied || len(evidence.ThumbnailHashes) == 0 {
		c.Remove(identity, ref)
		return
	}
	verified := make([]api.Item, 0, len(items))
	seenPaths := make(map[string]bool)
	for _, item := range items {
		image := item.Image
		if image == nil {
			continue
		}
		path := item.ID
		if item.File != nil {
			path = item.File.Path
		}
		if !filepath.IsAbs(path) {
			continue
		}
		path = filepath.Clean(path)
		denied := false
		for _, p := range evidence.DeniedPaths {
			if filepath.Clean(p) == path {
				denied = true
				break
			}
		}
		if denied {
			continue
		}
		hash := sha256.Sum256([]byte(image.Src))
		for _, p := range evidence.ThumbnailPaths[hash] {
			if p == path && !seenPaths[path] {
				safe := item
				safe.ID = path
				safe.Title = filepath.Base(path)
				safe.Subtitle = ""
				safe.Accessories = nil
				safe.Icon = ""
				safe.Glyph = ""
				picture := *image
				picture.Alt = ""
				safe.Image = &picture
				verified = append(verified, safe)
				seenPaths[path] = true
				break
			}
		}
	}
	if len(verified) == 0 {
		c.Remove(identity, ref)
		return
	}
	view.Title = "Cached files"
	view.Subtitle = ""
	view.Placeholder = ""
	view.Hint = ""
	view.Status = ""
	view.ID = ""
	view.SelectedItemID = ""
	c.Put(identity, ref, view, verified)
	key, ok := snapshotKey(identity, ref)
	if !ok {
		return
	}
	if _, ok := c.entries[key]; !ok {
		return
	}
	if c.sourcePaths == nil {
		c.sourcePaths = make(map[commandSnapshotKey]map[[32]byte]bool)
	}
	paths := make(map[[32]byte]bool, len(verified))
	for _, item := range verified {
		path := item.ID
		if item.File != nil {
			path = item.File.Path
		}
		paths[sha256.Sum256([]byte(filepath.Clean(path)))] = true
	}
	c.sourcePaths[key] = paths
}

// CommandSnapshots keeps bounded, process-local presentation snapshots. Identity belongs to the host,
// not the extension, and must change when the installed revision or its enablement changes.
type CommandSnapshots struct {
	entries     map[commandSnapshotKey]commandSnapshotEntry
	protected   map[commandSnapshotKey]struct{}
	staleHashes map[commandSnapshotKey]map[[32]byte]bool
	sourcePaths map[commandSnapshotKey]map[[32]byte]bool
	order       []commandSnapshotKey
	bytes       int
	nextVersion uint64
}

func (s *Session) clearPrivateSnapshot(failure *api.Failure) {
	s.rememberSnapshotImages()
	if s.cancelImages != nil {
		s.cancelImages()
	}
	s.items, s.sections = nil, nil
	s.view.Items, s.view.Sections = nil, nil
	s.view.Title, s.view.Subtitle, s.view.Status = "Access unavailable", "", ""
	s.view.Placeholder, s.view.Hint, s.view.SelectedItemID = "", "", ""
	s.view.Actions, s.view.ActionPanel = nil, nil
	s.selectID = ""
	s.view.EmptyView = nil
	s.guard.Lock()
	s.images = map[string]*ui.Bitmap{}
	s.requested = map[string]bool{}
	s.handles = map[handle]route{}
	s.filePreviews = nil
	s.guard.Unlock()
	s.loading, s.failure = false, failure
	if s.palettePage != nil {
		s.palettePage.Title, s.palettePage.Subtitle = s.view.Title, ""
		s.palettePage.Placeholder, s.palettePage.Hint = "", ""
		s.palettePage.EmptyTitle, s.palettePage.EmptySub = "No items", ""
	}
	s.refreshHost()
}

type commandSnapshotEntry struct {
	view    api.View
	size    int
	version uint64
}
type commandSnapshotKey struct {
	identity   any
	invocation string
}

const maxCommandSnapshotBytes = 16 << 20
const maxCommandSnapshots = 8
const maxProtectedCommands = 256

func sameSnapshotIdentity(a, b any) bool {
	return a != nil && b != nil && reflect.TypeOf(a).Comparable() && reflect.TypeOf(b).Comparable() && a == b
}

func snapshotKey(identity any, ref api.ActionRef) (commandSnapshotKey, bool) {
	if identity == nil || !reflect.TypeOf(identity).Comparable() {
		return commandSnapshotKey{}, false
	}
	data, err := json.Marshal(ref)
	if err != nil || len(data) > 4096 {
		return commandSnapshotKey{}, false
	}
	return commandSnapshotKey{identity, string(data)}, true
}

func (c *CommandSnapshots) Get(identity any, ref api.ActionRef) (api.View, bool) {
	key, ok := snapshotKey(identity, ref)
	if !ok {
		return api.View{}, false
	}
	entry, ok := c.entries[key]
	if !ok {
		return api.View{}, false
	}
	return cloneSnapshot(entry.view), true
}

func cloneSnapshot(view api.View) api.View {
	if view.Grid != nil {
		grid := *view.Grid
		view.Grid = &grid
	}
	if view.Refresh != nil {
		refresh := *view.Refresh
		if refresh.Ref != nil {
			ref := *refresh.Ref
			ref.Args = make(map[string]string, len(refresh.Ref.Args))
			for k, v := range refresh.Ref.Args {
				ref.Args[k] = v
			}
			refresh.Ref = &ref
		}
		view.Refresh = &refresh
	}
	view.Items = append([]api.Item(nil), view.Items...)
	for i := range view.Items {
		item := &view.Items[i]
		item.Accessories = append([]api.Accessory(nil), item.Accessories...)
		if item.Image != nil {
			image := *item.Image
			item.Image = &image
		}
	}
	return view
}

func (c *CommandSnapshots) remove(key commandSnapshotKey) {
	if old, ok := c.entries[key]; ok {
		c.bytes -= old.size
		delete(c.entries, key)
		for i, k := range c.order {
			if k == key {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
	}
}

func (c *CommandSnapshots) Remove(identity any, ref api.ActionRef) {
	if key, ok := snapshotKey(identity, ref); ok {
		c.remove(key)
	}
}

func (c *CommandSnapshots) Version(identity any, ref api.ActionRef) uint64 {
	key, ok := snapshotKey(identity, ref)
	if !ok {
		return 0
	}
	return c.entries[key].version
}

func (c *CommandSnapshots) RemoveVersion(identity any, ref api.ActionRef, version uint64) {
	key, ok := snapshotKey(identity, ref)
	if ok && version != 0 && c.entries[key].version == version {
		c.remove(key)
	}
}

func (c *CommandSnapshots) IsProtected(identity any, ref api.ActionRef) bool {
	key, ok := snapshotKey(identity, ref)
	if !ok {
		return false
	}
	_, ok = c.protected[key]
	return ok
}

func (c *CommandSnapshots) StaleHashes(identity any, ref api.ActionRef) map[[32]byte]bool {
	key, ok := snapshotKey(identity, ref)
	if !ok {
		return nil
	}
	return c.staleHashes[key]
}

// CoversDenial binds invalidation to files which actually supplied the snapshot.
func (c *CommandSnapshots) CoversDenial(identity any, ref api.ActionRef, denial *api.SnapshotDenial) bool {
	if denial == nil {
		return false
	}
	if denial.AuthorityDenied {
		return true
	}
	key, ok := snapshotKey(identity, ref)
	if !ok || !filepath.IsAbs(denial.Path) {
		return false
	}
	return c.sourcePaths[key][sha256.Sum256([]byte(filepath.Clean(denial.Path)))]
}

func (c *CommandSnapshots) SnapshotPaths(identity any, ref api.ActionRef) map[[32]byte]bool {
	key, ok := snapshotKey(identity, ref)
	if !ok {
		return nil
	}
	return c.sourcePaths[key]
}

func snapshotPathsCover(paths map[[32]byte]bool, denial *api.SnapshotDenial) bool {
	if denial == nil || len(paths) == 0 {
		return false
	}
	if denial.AuthorityDenied {
		return true
	}
	if !filepath.IsAbs(denial.Path) {
		return false
	}
	return paths[sha256.Sum256([]byte(filepath.Clean(denial.Path)))]
}

func verifiedSnapshotPaths(items []api.Item, evidence *api.SnapshotEvidence) map[[32]byte]bool {
	paths := make(map[[32]byte]bool)
	if evidence == nil || evidence.FilesAuthorityDenied {
		return paths
	}
	for _, item := range items {
		if item.Image == nil {
			continue
		}
		path := item.ID
		if item.File != nil {
			path = item.File.Path
		}
		if !filepath.IsAbs(path) {
			continue
		}
		path = filepath.Clean(path)
		denied := false
		for _, p := range evidence.DeniedPaths {
			if filepath.Clean(p) == path {
				denied = true
				break
			}
		}
		if denied {
			continue
		}
		hash := sha256.Sum256([]byte(item.Image.Src))
		for _, p := range evidence.ThumbnailPaths[hash] {
			if p == path {
				paths[sha256.Sum256([]byte(path))] = true
				break
			}
		}
	}
	return paths
}

// snapshotImageSource keeps an embedded PNG or the host's thumbnail reference, which is inert until drawn.
func snapshotImageSource(src string) bool {
	return strings.HasPrefix(src, "data:image/png;base64,") || thumbs.ReferenceEdge(src) == thumbs.GridEdge
}

// Put stores only inert presentation, never actions, paths, or external image references.
func (c *CommandSnapshots) Put(identity any, ref api.ActionRef, view api.View, items []api.Item) {
	key, ok := snapshotKey(identity, ref)
	if !ok {
		return
	}
	if view.Kind != api.ViewGrid || view.Refresh == nil || view.Refresh.Ref == nil || !view.Refresh.Immediate || len(items) == 0 || len(items) > 200 || len(view.Sections) != 0 || view.Source != nil || view.Pagination != nil || view.SearchAccessory != nil || view.OnSelectionChange != nil || view.OnSearchChange != nil || view.Search != nil || len(view.Actions) != 0 || view.ActionPanel != nil || len(view.FooterHints) != 0 {
		c.remove(key)
		return
	}
	refreshRef, err := json.Marshal(view.Refresh.Ref)
	if err != nil || len(refreshRef) > 4096 {
		c.remove(key)
		return
	}
	clean := api.View{ID: view.ID, Kind: view.Kind, Title: view.Title, Subtitle: view.Subtitle, Size: view.Size, Presentation: view.Presentation, ContentSizing: view.ContentSizing, WindowCompact: view.WindowCompact, Placeholder: view.Placeholder, Status: view.Status, Grid: view.Grid, Refresh: view.Refresh, MaxVisibleItems: view.MaxVisibleItems, SelectedItemID: view.SelectedItemID, Hint: view.Hint}
	for _, item := range items {
		row := api.Item{ID: item.ID, Title: item.Title, Subtitle: item.Subtitle, Icon: item.Icon, Glyph: item.Glyph, Accessories: item.Accessories}
		if item.Image != nil && snapshotImageSource(item.Image.Src) {
			image := *item.Image
			image.Light, image.Dark, image.Fallback = "", "", ""
			row.Image = &image
		}
		clean.Items = append(clean.Items, row)
	}
	data, err := json.Marshal(clean)
	if err != nil || len(data) > maxCommandSnapshotBytes {
		c.remove(key)
		return
	}
	if c.entries == nil {
		c.entries = make(map[commandSnapshotKey]commandSnapshotEntry)
	}
	if c.protected == nil {
		c.protected = make(map[commandSnapshotKey]struct{})
	}
	if _, exists := c.protected[key]; !exists && len(c.protected) >= maxProtectedCommands {
		c.remove(key)
		return
	}
	c.protected[key] = struct{}{}
	if c.staleHashes == nil {
		c.staleHashes = make(map[commandSnapshotKey]map[[32]byte]bool)
	}
	hashes := make(map[[32]byte]bool)
	for _, item := range clean.Items {
		if item.Image != nil && item.Image.Src != "" {
			hashes[sha256.Sum256([]byte(item.Image.Src))] = true
		}
	}
	c.staleHashes[key] = hashes
	c.remove(key)
	for len(c.order) >= maxCommandSnapshots || c.bytes+len(data) > maxCommandSnapshotBytes {
		oldest := c.order[0]
		c.order = c.order[1:]
		c.bytes -= c.entries[oldest].size
		delete(c.entries, oldest)
	}
	c.nextVersion++
	c.entries[key] = commandSnapshotEntry{view: cloneSnapshot(clean), size: len(data), version: c.nextVersion}
	c.order = append(c.order, key)
	c.bytes += len(data)
}
