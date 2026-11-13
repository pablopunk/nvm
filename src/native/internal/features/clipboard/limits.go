package clipboard

import "time"

// Default retention and capture limits. They bound memory and disk for a history of thousands of items.
const (
	// DefaultMaxEntries is how many entries are kept.
	DefaultMaxEntries = 300
	// DefaultMaxAge expires entries older than this.
	DefaultMaxAge = 30 * 24 * time.Hour
	// DefaultMaxImageBytes caps the PNG bytes of image entries on disk.
	DefaultMaxImageBytes = 256 << 20
	// DefaultMaxMemoryBytes caps the text and markup held in memory by entries.
	DefaultMaxMemoryBytes = 32 << 20
	// DefaultLimit is how many rows a query returns when the caller sets no limit.
	DefaultLimit = 300
	// MaxCaptureTextBytes drops a copy whose text is larger than this, before it is stored.
	MaxCaptureTextBytes = 1 << 20
	// MaxCaptureImageBytes drops a copied PNG larger than this, before it is decoded.
	MaxCaptureImageBytes = 16 << 20
	// MaxImagePixels drops an image whose decoded size is larger than this, a guard against decompression
	// bombs. At four bytes a pixel the transient decode for one image stays near 64 MB.
	MaxImagePixels = 16_000_000
	// SuppressWindow is how long a copy made by the app itself is ignored by the watcher.
	SuppressWindow = 2 * time.Second
)
