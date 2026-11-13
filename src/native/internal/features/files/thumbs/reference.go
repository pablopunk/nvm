package thumbs

import "net/url"

// GridEdge bounds extension grid pictures. A tile of a large grid is wider than 600 pixels on a Retina display, so a grid
// picture is rendered at preview size: anything smaller looks soft.
const GridEdge = PreviewEdge

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

// IsGridReference reports whether ref is a reference GridReference made.
func IsGridReference(ref string) bool {
	u, err := url.Parse(ref)
	return err == nil && u.Scheme == "file" && u.Fragment == thumbnailFragment
}

// ReferenceEdge is the size a file reference is rendered at.
func ReferenceEdge(ref string) int {
	if IsGridReference(ref) {
		return GridEdge
	}
	return PreviewEdge
}
