package platform

import "context"

// FilePictures is the intent "draw a picture of any file": an image, a video frame, the first page of a document, or the
// icon of its kind. A platform that cannot leaves the port nil.
type FilePictures interface {
	// FilePicture returns a PNG that fits inside an edge-by-edge square.
	FilePicture(ctx context.Context, path string, edge int) ([]byte, error)
}
