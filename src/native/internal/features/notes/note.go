package notes

import (
	"regexp"
	"strconv"
	"time"
)

// Limits the 1.x extension enforced: 500 notes and 512 KiB of text per note. Longer text is cut.
const (
	MaxNotes     = 500
	MaxTextBytes = 524_288
)

// WriteDelay is how long a store change waits before its document is written. Bursts of saves become one write.
const WriteDelay = 300 * time.Millisecond

var validNoteIDPattern = regexp.MustCompile(`^note-[a-z0-9-]+$`)

// Note is one Markdown note.
type Note struct {
	ID        string    `json:"id"`
	Markdown  string    `json:"markdown"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Version is the saved version of a note: its UpdatedAt in nanoseconds. A save made on top of a different version is
// a conflict, not an overwrite.
func (n Note) Version() int64 {
	return n.UpdatedAt.UnixNano()
}

// validNoteID reports whether id has the shape notes are given.
func validNoteID(id string) bool {
	return validNoteIDPattern.MatchString(id)
}

// newNoteID returns a note id that sorts by creation time and is unique within one store.
func newNoteID(now time.Time, sequence uint64) string {
	return "note-" + strconv.FormatInt(now.UnixNano(), 36) + "-" + strconv.FormatUint(sequence, 36)
}

// nextSavedTime returns the UpdatedAt of a save at now. It is later than previous, so a save in the same clock
// tick still changes the version the editor is checked against.
func nextSavedTime(now time.Time, previous int64) time.Time {
	if previous != 0 && now.UnixNano() <= previous {
		return time.Unix(0, previous+1).UTC()
	}
	return now
}

// truncateText keeps at most MaxTextBytes of text, cut at a rune boundary.
func truncateText(text string) string {
	if len(text) <= MaxTextBytes {
		return text
	}
	cut := MaxTextBytes
	for cut > 0 && text[cut]&0xC0 == 0x80 {
		cut--
	}
	return text[:cut]
}
