package api

// FileRef describes a local file. It is returned by the files service and accepted by file rows
// and file-preview views. Timestamps are epoch milliseconds: one encoding, no ISO duplicates.
type FileRef struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	DisplayPath string `json:"displayPath,omitempty"`
	// URL is a thumbnail-safe display URL for palette views.
	URL          string `json:"url,omitempty"`
	FileURL      string `json:"fileUrl,omitempty"`
	VideoURL     string `json:"videoUrl,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	// Kind is "image", "video", "file" or another host-recognised kind.
	Kind      string `json:"kind,omitempty"`
	Extension string `json:"extension,omitempty"`
	MimeType  string `json:"mimeType,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Size      int64  `json:"size,omitempty"`
	MtimeMs   int64  `json:"mtimeMs,omitempty"`
	// BirthtimeMs is the creation time where the OS reports it.
	BirthtimeMs int64 `json:"birthtimeMs,omitempty"`
	// DateAddedMs is when the file arrived in its folder (macOS "Date Added"), where known.
	DateAddedMs int64 `json:"dateAddedMs,omitempty"`
}

// IndexedFile is an entry from the host file index. It is lighter than FileRef and never
// rescans the disk when read.
type IndexedFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	DisplayPath  string `json:"displayPath,omitempty"`
	Extension    string `json:"extension,omitempty"`
	Kind         string `json:"kind,omitempty"`
	URL          string `json:"url,omitempty"`
	FileURL      string `json:"fileUrl,omitempty"`
	VideoURL     string `json:"videoUrl,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	MtimeMs      int64  `json:"mtimeMs,omitempty"`
	BirthtimeMs  int64  `json:"birthtimeMs,omitempty"`
	DateAddedMs  int64  `json:"dateAddedMs,omitempty"`
	Size         int64  `json:"size,omitempty"`
}
