package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// FindImages walks the roots for pictures. The kind of options is replaced, so the call is always the image kind.
func (f remoteFiles) FindImages(roots []string, options api.FindOptions) ([]api.FileRef, error) {
	options.Kind = "image"
	return f.Find(roots, options)
}

// FindVideos walks the roots for videos; see FindImages.
func (f remoteFiles) FindVideos(roots []string, options api.FindOptions) ([]api.FileRef, error) {
	options.Kind = "video"
	return f.Find(roots, options)
}

// FindMedia walks the roots for pictures and videos; see FindImages.
func (f remoteFiles) FindMedia(roots []string, options api.FindOptions) ([]api.FileRef, error) {
	options.Kind = "media"
	return f.Find(roots, options)
}

// QuickLook shows the file in the system's previewer, outside Nevermind.
func (f remoteFiles) QuickLook(path string) error {
	return f.link.Call(protocol.ServiceDesktopFiles, protocol.OpQuickLook, protocol.PathArgs{Path: path}, nil)
}

// ReadText returns the whole text of a file of at most 256 KiB.
func (f remoteFiles) ReadText(path string) (string, error) {
	var text string
	err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpReadText, protocol.PathArgs{Path: path}, &text)
	return text, err
}

// WriteText saves UTF-8 text into an existing folder; see the api contract.
func (f remoteFiles) WriteText(path, text string, overwrite bool) (api.FileRef, error) {
	var ref api.FileRef
	err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpWriteText, protocol.WriteFileTextArgs{Path: path, Text: text, Overwrite: overwrite}, &ref)
	return ref, err
}

// DisplayPath formats a path with the home folder as "~". It returns the path unchanged when the host cannot answer.
func (f remoteFiles) DisplayPath(path string) string {
	var display string
	if err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpDisplayPath, protocol.PathArgs{Path: path}, &display); err != nil {
		return path
	}
	return display
}

// IndexedRoots lists the default roots of the file index.
func (f remoteFiles) IndexedRoots() []string {
	var roots []string
	if err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpIndexedRoots, nil, &roots); err != nil {
		return nil
	}
	return roots
}

// IndexSnapshot reads the index, filtered and sorted, without rescanning. Failures answer nothing.
func (f remoteFiles) IndexSnapshot(options api.IndexOptions) []api.IndexedFile {
	var files []api.IndexedFile
	if err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpIndexSnapshot, protocol.IndexArgs{Options: options}, &files); err != nil {
		return nil
	}
	return files
}

// Reindex starts a rescan of the index with the bounded options and reports the count and roots.
func (f remoteFiles) Reindex(options api.IndexOptions) (api.IndexReport, error) {
	var report api.IndexReport
	err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpReindex, protocol.IndexArgs{Options: options}, &report)
	return report, err
}

// Recent lists the index's newest files first. Failures answer nothing.
func (f remoteFiles) Recent(options api.IndexOptions) []api.IndexedFile {
	var files []api.IndexedFile
	if err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpIndexRecent, protocol.IndexArgs{Options: options}, &files); err != nil {
		return nil
	}
	return files
}

// SearchIndex lists the index entries whose name or path matches the query. Failures answer nothing.
func (f remoteFiles) SearchIndex(query string, options api.IndexOptions) []api.IndexedFile {
	var files []api.IndexedFile
	if err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpSearchIndex, protocol.SearchIndexArgs{Query: query, Options: options}, &files); err != nil {
		return nil
	}
	return files
}
