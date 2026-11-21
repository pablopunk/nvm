package clipboard

import "time"

// Kind names what a history entry holds.
type Kind string

// Entry kinds. Rich text keeps its plain text next to the original markup, so search works on the text and
// a paste can restore the formatting.
const (
	KindText  Kind = "text"
	KindHTML  Kind = "html"
	KindRTF   Kind = "rtf"
	KindImage Kind = "image"
	KindFiles Kind = "files"
)

// Valid reports whether k is one of the known kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindText, KindHTML, KindRTF, KindImage, KindFiles:
		return true
	}
	return false
}

// Entry is one clipboard history item. ID is a stable content hash, so the same content copied twice has the
// same ID. Concealed is never persisted: History refuses an entry that sets it, so a concealed copy cannot be
// stored, logged or announced.
type Entry struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	// Text is the plain text. For files it is the paths joined by newlines, for search.
	Text string `json:"text,omitempty"`
	// Original is the HTML or RTF markup of a rich entry.
	Original string `json:"original,omitempty"`
	// Paths lists the files of a files entry, absolute, in the order copied.
	Paths []string `json:"paths,omitempty"`
	// ImageRef is the file name of an image entry inside the image store, such as 40 hex digits and .png.
	ImageRef string `json:"imageRef,omitempty"`
	// Hash is the content hash of an image entry, the stem of its file name.
	Hash   string `json:"hash,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	// Bytes is the size of the stored PNG of an image entry.
	Bytes     int64     `json:"bytes,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	// UsedAt is the last time the entry was copied or pasted from history. It informs ranking later and does
	// not change the list order.
	UsedAt    time.Time `json:"usedAt"`
	SourceApp string    `json:"sourceApp,omitempty"`
	// LinkPreview is set once a page's metadata is known. Candidacy is decided from Text by SingleWebURL.
	LinkPreview *LinkPreview `json:"linkPreview,omitempty"`
	Concealed   bool         `json:"-"`
}

// LinkPreview is the stored result of fetching a copied web address. It is data only; the fetch lives in
// LinkService and its network implementation.
type LinkPreview struct {
	URL         string    `json:"url"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	SiteName    string    `json:"siteName,omitempty"`
	ImageRef    string    `json:"imageRef,omitempty"`
	FetchedAt   time.Time `json:"fetchedAt"`
}

// imageBytes is the disk size an entry adds to the image budget.
func (e Entry) imageBytes() int64 {
	if e.Kind != KindImage {
		return 0
	}
	return e.Bytes
}

// memoryBytes is the text an entry holds in memory: plain text, markup and paths.
func (e Entry) memoryBytes() int64 {
	size := int64(len(e.Text) + len(e.Original))
	for _, path := range e.Paths {
		size += int64(len(path))
	}
	return size
}
