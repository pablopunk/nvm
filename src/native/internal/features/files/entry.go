package files

// Kind classifies a file by its extension. KindMedia is a query-only value that matches images and videos.
type Kind uint8

// File kinds. Entries carry KindFile, KindImage or KindVideo; queries may also ask for KindMedia.
const (
	KindFile Kind = iota
	KindImage
	KindVideo
	KindMedia
)

// String returns the name the extension API uses for the kind: file, image, video or media.
func (k Kind) String() string {
	switch k {
	case KindImage:
		return "image"
	case KindVideo:
		return "video"
	case KindMedia:
		return "media"
	default:
		return "file"
	}
}

// entry is one indexed file. It is 40 bytes on 64-bit platforms: the name lives in the snapshot's name arena
// and the directory in its interned directory table, so an entry holds only offsets and the metadata the
// palette sorts and shows. Times are Unix milliseconds, as file-index.ts reports them.
type entry struct {
	dir     uint32 // index into Snapshot.dirs
	nameOff uint32 // offset of the name in Snapshot.names
	nameLen uint16 // length of the name in bytes
	ext     uint16 // index into Snapshot.exts; the lowercased extension without the dot
	kind    Kind
	size    int64
	modMs   int64
	// createdMs is the creation time when the platform reports one and the modification time otherwise. Until
	// the platform supplies a separate "date added", the "added" and "created" sorts share this column.
	createdMs int64
}

// entrySize is the byte size of one entry, asserted by the tests so a field change cannot grow it silently.
const entrySize = 40
