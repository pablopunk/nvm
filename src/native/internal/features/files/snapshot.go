package files

import (
	"path/filepath"
	"strings"
	"time"
)

// Snapshot is an immutable view of the index. Readers hold a snapshot for the length of one query; the index
// publishes a new one by swapping a pointer, so no reader ever sees a half-built index. A snapshot stores each
// directory path once in dirs, each extension once in exts and every name in one arena, which keeps the
// per-file cost to the 40-byte entry plus the name bytes.
type Snapshot struct {
	generation uint64
	configKey  string
	dirs       []string
	exts       []string
	names      []byte
	entries    []entry
}

// Len returns the number of indexed files.
func (s *Snapshot) Len() int { return len(s.entries) }

// Generation counts the snapshots an index has published since it started, so consumers can detect a change.
func (s *Snapshot) Generation() uint64 { return s.generation }

// nameOf returns the name bytes of an entry. The slice aliases the arena and must not be modified.
func (s *Snapshot) nameOf(e entry) []byte {
	return s.names[e.nameOff : e.nameOff+uint32(e.nameLen)]
}

// Path returns the full path of the file at index i. Folder paths are stored clean, so the path is one concatenation.
func (s *Snapshot) Path(i int) string {
	e := s.entries[i]
	dir := s.dirs[e.dir]
	name := s.nameOf(e)
	if strings.HasSuffix(dir, string(filepath.Separator)) {
		return dir + string(name)
	}
	return dir + string(filepath.Separator) + string(name)
}

// Name returns the file name at index i.
func (s *Snapshot) Name(i int) string { return string(s.nameOf(s.entries[i])) }

// Info describes one file of a snapshot without its path.
type Info struct {
	Name      string
	Extension string
	Kind      Kind
	Size      int64
	Modified  time.Time
	Created   time.Time
}

// Info returns the metadata of the file at index i.
func (s *Snapshot) Info(i int) Info {
	e := s.entries[i]
	return Info{
		Name:      string(s.nameOf(e)),
		Extension: s.exts[e.ext],
		Kind:      e.kind,
		Size:      e.size,
		Modified:  millisToTime(e.modMs),
		Created:   millisToTime(e.createdMs),
	}
}

// millisToTime converts Unix milliseconds to a time, mapping the zero value to the zero time.
func millisToTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
