package thumbs

import "net/url"

// GridEdge bounds extension grid pictures independently of full file previews.
const GridEdge = 256

const thumbnailFragment = "nevermind-thumbnail"

// GridReference marks a file URL for thumbnail rendering. The fragment changes
// the image-cache identity, never the filesystem path or access permissions.
func GridReference(fileURL string) string {
	u, err := url.Parse(fileURL)
	if err != nil || u.Scheme != "file" {
		return fileURL
	}
	u.Fragment = thumbnailFragment
	return u.String()
}

// ReferenceEdge keeps ordinary file references at preview resolution. Only the
// thumbnail service's rendering hint requests the smaller grid size.
func ReferenceEdge(ref string) int {
	u, err := url.Parse(ref)
	if err == nil && u.Scheme == "file" && u.Fragment == thumbnailFragment {
		return GridEdge
	}
	return PreviewEdge
}
