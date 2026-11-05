package api

// FindOptions filters a directory walk. Kind may be image, video, media or file. Scanning stops at
// ScanLimit candidates before sorting, so huge roots return approximate newest results.
type FindOptions struct {
	Limit             int      `json:"limit,omitempty"`
	Depth             int      `json:"depth,omitempty"`
	Extensions        []string `json:"extensions,omitempty"`
	Kind              string   `json:"kind,omitempty"`
	Patterns          []string `json:"patterns,omitempty"`
	SortBy            string   `json:"sortBy,omitempty"` // recent, modified, added, created, name or size
	Order             string   `json:"order,omitempty"`  // asc or desc
	IncludeDimensions bool     `json:"includeDimensions,omitempty"`
	ScanLimit         int      `json:"scanLimit,omitempty"`
}

// IndexOptions filters the host file index. Query filters an existing snapshot.
type IndexOptions struct {
	FindOptions
	Roots         []string `json:"roots,omitempty"`
	IncludeHidden bool     `json:"includeHidden,omitempty"`
	Ignore        []string `json:"ignore,omitempty"`
	Query         string   `json:"query,omitempty"`
}

// IndexReport is returned by Files.Reindex.
type IndexReport struct {
	Count int      `json:"count"`
	Roots []string `json:"roots"`
}

// IndexStatus describes the file index: Ready once it holds a snapshot, Scanning while a rescan runs.
type IndexStatus struct {
	Ready        bool  `json:"ready"`
	Scanning     bool  `json:"scanning"`
	Entries      int   `json:"entries"`
	ScannedFiles int64 `json:"scannedFiles"`
}

// Files finds, reads, writes and previews local files. Paths may start with "~".
type Files interface {
	// Find walks roots. Missing roots are skipped with a logged warning, so one batch covers them all.
	Find(roots []string, options FindOptions) ([]FileRef, error)
	// FindImages, FindVideos and FindMedia are Find with the kind fixed to image, video or media. The kind in options
	// is ignored.
	FindImages(roots []string, options FindOptions) ([]FileRef, error)
	FindVideos(roots []string, options FindOptions) ([]FileRef, error)
	FindMedia(roots []string, options FindOptions) ([]FileRef, error)
	OpenWithApps(path string) ([]App, error)
	Open(path string) error
	Reveal(path string) error
	// QuickLook opens the OS previewer outside Nevermind; check System().HasCapability("quick-look").
	QuickLook(path string) error
	ReadText(path string) (string, error)
	// WriteText fails when the file exists unless overwrite is set. It never creates folders.
	WriteText(path, text string, overwrite bool) (FileRef, error)
	// WriteBytes saves raw bytes, such as a downloaded image. It fails when the file exists unless overwrite is set,
	// never creates folders and replaces the file atomically. The host accepts at most 512 KiB per call.
	WriteBytes(path string, data []byte, overwrite bool) (FileRef, error)
	DisplayPath(path string) string
	FileURL(path string) string
	Thumbnail(path string) (url string, found bool)
	Metadata(path string) (FileRef, error)
	IndexedRoots() []string
	IndexSnapshot(options IndexOptions) []IndexedFile
	Reindex(options IndexOptions) (IndexReport, error)
	Recent(options IndexOptions) []IndexedFile
	SearchIndex(query string, options IndexOptions) []IndexedFile
	// IndexStatus reports whether the index is ready and scanning.
	IndexStatus() IndexStatus
}
