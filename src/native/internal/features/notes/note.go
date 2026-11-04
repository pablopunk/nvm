package notes

import (
	"regexp"
	"strconv"
	"time"
)

// Limits the 1.x extension enforced: 500 notes and 512 KiB of text per note or draft. Longer text is cut.
const (
	MaxNotes     = 500
	MaxTextBytes = 524_288
)

// The size of a floating note window when it has no remembered frame: the 1.x floating window's size.
const (
	defaultWidth  = 640
	defaultHeight = 460
)

// AutosaveDelay is how long the editor waits after the last keystroke before it saves the note.
const AutosaveDelay = 400 * time.Millisecond

// DraftDelay is how long a keystroke batch waits before it is written to the draft document. The store that
// holds the documents uses it as its debounce.
const DraftDelay = 300 * time.Millisecond

var validNoteIDPattern = regexp.MustCompile(`^note-[a-z0-9-]+$`)

// Frame is the position and size of a floating note window, in DIPs.
type Frame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Note is one Markdown note. Floating says whether it is the note the floating window shows.
type Note struct {
	ID        string    `json:"id"`
	Markdown  string    `json:"markdown"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Floating  bool      `json:"floating,omitempty"`
}

// Version is the saved version of a note: its UpdatedAt in nanoseconds. Drafts record the version they were
// based on, and a different saved version is what makes a conflict.
func (n Note) Version() int64 {
	return n.UpdatedAt.UnixNano()
}

// Title is the note's title, derived from its first content line.
func (n Note) Title() string {
	return Title(n.Markdown)
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
// tick still changes the version the drafts are checked against.
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
