package platform

import "context"

// FilePictures is the intent "draw a picture of any file": an image, a video frame, the first page of a document, or the
// icon of its kind. A platform that cannot leaves the port nil.
type FilePictures interface {
	// FilePicture returns a PNG that fits inside an edge-by-edge square.
	FilePicture(ctx context.Context, path string, edge int) ([]byte, error)
}

// IconLooks is the intent "draw icons in the look the system has now". macOS tints icons to the user's choice, and a
// running app keeps drawing the look it launched with. A FilePictures may also be one.
type IconLooks interface {
	// IconLook names the look now, and is never empty. It also reports whether that differs from the look at launch.
	IconLook(ctx context.Context) (look string, changed bool)
	// ThemedIcon returns the icon of path in the look now, as a PNG of size by size pixels.
	ThemedIcon(ctx context.Context, path string, size int) ([]byte, error)
}
