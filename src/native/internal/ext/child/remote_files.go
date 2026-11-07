package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteFiles finds, describes, previews and opens local files on the host. Text writes, previews and the file index
// are not served yet.
type remoteFiles struct {
	api.Files
	link *hostLink
}

func (f remoteFiles) Open(path string) error {
	return f.link.Call(protocol.ServiceDesktopFiles, protocol.OpOpen, protocol.PathArgs{Path: path}, nil)
}

func (f remoteFiles) Reveal(path string) error {
	return f.link.Call(protocol.ServiceDesktopFiles, protocol.OpReveal, protocol.PathArgs{Path: path}, nil)
}

func (f remoteFiles) Find(roots []string, options api.FindOptions) ([]api.FileRef, error) {
	var refs []api.FileRef
	err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpFind, protocol.FindArgs{Roots: roots, Options: options}, &refs)
	return refs, err
}

func (f remoteFiles) Metadata(path string) (api.FileRef, error) {
	var ref api.FileRef
	err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpMetadata, protocol.PathArgs{Path: path}, &ref)
	return ref, err
}

// Thumbnail reports no picture when the host cannot make one; the signature has no error to return.
func (f remoteFiles) Thumbnail(path string) (string, bool) {
	var result protocol.ThumbnailResult
	if err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: path}, &result); err != nil {
		return "", false
	}
	return result.URL, result.Found
}

// FileURL returns an empty string when the host cannot name the path; the signature has no error to return.
func (f remoteFiles) FileURL(path string) string {
	var url string
	if err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpFileURL, protocol.PathArgs{Path: path}, &url); err != nil {
		return ""
	}
	return url
}

func (f remoteFiles) WriteBytes(path string, data []byte, overwrite bool) (api.FileRef, error) {
	var ref api.FileRef
	err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpWriteBytes, protocol.WriteBytesArgs{Path: path, Data: data, Overwrite: overwrite}, &ref)
	return ref, err
}

func (f remoteFiles) OpenWithApps(path string) ([]api.App, error) {
	var apps []api.App
	err := f.link.Call(protocol.ServiceDesktopFiles, protocol.OpOpenWithApps, protocol.PathArgs{Path: path}, &apps)
	return apps, err
}
