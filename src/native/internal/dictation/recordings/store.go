// Package recordings keeps the audio of a dictation from its first captured frame until its transcript is in, so a
// dictation that failed, was cancelled or was cut off by a crash can be transcribed again from the history. Each
// dictation is a JSON record and one WebM file per minute of speech, written under the data folder while the user
// speaks. The record is written before the first segment and each segment replaces its earlier copy atomically, so a
// crash at any moment leaves every segment written so far listed and readable. Nothing is pruned: a recording stays
// until its transcript is in or the user deletes it.
package recordings

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// maxSegments is the longest recording the store takes: ten hours of one-minute segments.
const maxSegments = 600

// ErrInvalidID is returned for a recording name that is not a dictation's.
var ErrInvalidID = errors.New("dictation recordings: invalid recording id")

// ErrNotFound is returned when no saved recording has the id.
var ErrNotFound = errors.New("dictation recordings: recording not found")

// ErrSegmentCount is returned for a recording with no segments, or a segment index past maxSegments.
var ErrSegmentCount = errors.New("dictation recordings: a recording needs between one and 600 segments")

var idPattern = regexp.MustCompile(`^dictation-[A-Za-z0-9-]{1,80}$`)

// Recording describes one saved dictation.
type Recording struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"createdAt"`
	// SegmentCount is how many segments are on disk, counted when the record is read.
	SegmentCount int `json:"segmentCount"`
	// Failure says why the last transcription failed, in the words the user reads. Empty for a dictation that was cut
	// off (the app closed while it was recording or on its way) and never got an answer.
	Failure string `json:"failure,omitempty"`
}

// Store reads and writes the saved dictations in dir. It is safe to use from several goroutines when the file port is,
// as long as one dictation is written by one goroutine at a time.
type Store struct {
	files platform.FS
	dir   string
	now   func() time.Time
}

// New returns a store over the folder dir. now stamps each recording, so tests control the order.
func New(files platform.FS, dir string, now func() time.Time) *Store {
	return &Store{files: files, dir: dir, now: now}
}

// SaveSegment writes segment index of the dictation id, replacing an earlier copy of it. The first segment of a
// dictation writes its record first, so the dictation is listed from its first frame.
func (s *Store) SaveSegment(id string, index int, audio []byte) error {
	if err := validID(id); err != nil {
		return err
	}
	if index < 0 || index >= maxSegments {
		return ErrSegmentCount
	}
	if err := s.files.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	if _, err := s.readRecord(id); errors.Is(err, ErrNotFound) {
		if err := s.writeRecord(Recording{ID: id, CreatedAt: s.now().UnixMilli()}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return s.replaceFile(s.segmentPath(id, index), audio)
}

// Save writes every segment of the dictation id, as SaveSegment does one by one.
func (s *Store) Save(id string, segments [][]byte) error {
	if len(segments) == 0 || len(segments) > maxSegments {
		return ErrSegmentCount
	}
	for index, audio := range segments {
		if err := s.SaveSegment(id, index, audio); err != nil {
			return err
		}
	}
	return nil
}

// MarkFailed records why the transcription of the saved dictation id failed, so the history can say so after a restart.
func (s *Store) MarkFailed(id, failure string) error {
	recording, err := s.readRecord(id)
	if err != nil {
		return err
	}
	recording.Failure = failure
	return s.writeRecord(recording)
}

// List returns the saved recordings that hold audio, newest first. A record with no segment yet is skipped.
func (s *Store) List() ([]Recording, error) {
	entries, err := s.files.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var recordings []Recording
	for _, entry := range entries {
		id, isRecord := strings.CutSuffix(entry.Name(), ".json")
		if !isRecord {
			continue
		}
		if recording, err := s.readRecord(id); err == nil && recording.SegmentCount > 0 {
			recordings = append(recordings, recording)
		}
	}
	slices.SortFunc(recordings, func(a, b Recording) int { return cmp.Compare(b.CreatedAt, a.CreatedAt) })
	return recordings, nil
}

// Load returns the segments of the saved dictation id, in order.
func (s *Store) Load(id string) ([][]byte, error) {
	recording, err := s.readRecord(id)
	if err != nil {
		return nil, err
	}
	if recording.SegmentCount == 0 {
		return nil, ErrNotFound
	}
	segments := make([][]byte, 0, recording.SegmentCount)
	for index := range recording.SegmentCount {
		audio, err := s.files.ReadFile(s.segmentPath(id, index))
		if err != nil {
			return nil, err
		}
		segments = append(segments, audio)
	}
	return segments, nil
}

// Remove deletes the saved dictation id: its segments last first, then its record, so a removal cut short leaves a
// shorter recording the user can still delete, never segments nobody lists. A recording that is already gone is not an
// error.
func (s *Store) Remove(id string) error {
	if err := validID(id); err != nil {
		return err
	}
	indexes, err := s.segmentIndexes(id)
	if err != nil {
		return err
	}
	for _, index := range slices.Backward(indexes) {
		if err := s.removeFile(s.segmentPath(id, index)); err != nil {
			return err
		}
	}
	return s.removeFile(s.recordPath(id))
}

var segmentPattern = regexp.MustCompile(`^(dictation-[A-Za-z0-9-]{1,80})\.(\d+)\.webm$`)

// segmentIndexes lists the indexes of every segment file of id on disk, in ascending order.
func (s *Store) segmentIndexes(id string) ([]int, error) {
	entries, err := s.files.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var indexes []int
	for _, entry := range entries {
		match := segmentPattern.FindStringSubmatch(entry.Name())
		if match == nil || match[1] != id {
			continue
		}
		if index, err := strconv.Atoi(match[2]); err == nil {
			indexes = append(indexes, index)
		}
	}
	slices.Sort(indexes)
	return indexes, nil
}

// readRecord reads and checks the record of id, and counts its segments: the unbroken run of segment files from the
// first one.
func (s *Store) readRecord(id string) (Recording, error) {
	if err := validID(id); err != nil {
		return Recording{}, err
	}
	data, err := s.files.ReadFile(s.recordPath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return Recording{}, ErrNotFound
	}
	if err != nil {
		return Recording{}, err
	}
	var recording Recording
	if err := json.Unmarshal(data, &recording); err != nil || recording.ID != id {
		return Recording{}, ErrNotFound
	}
	recording.SegmentCount = 0
	for recording.SegmentCount < maxSegments {
		if _, err := s.files.Stat(s.segmentPath(id, recording.SegmentCount)); err != nil {
			break
		}
		recording.SegmentCount++
	}
	return recording, nil
}

// writeRecord replaces the record of recording atomically.
func (s *Store) writeRecord(recording Recording) error {
	recording.SegmentCount = 0
	data, err := json.Marshal(recording)
	if err != nil {
		return err
	}
	return s.replaceFile(s.recordPath(recording.ID), data)
}

// replaceFile writes data beside name and renames it into place, so a crash never leaves half a file under name.
func (s *Store) replaceFile(name string, data []byte) error {
	temporary := name + ".tmp"
	if err := s.files.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return s.files.Rename(temporary, name)
}

// removeFile deletes name, treating a file that is already gone as deleted.
func (s *Store) removeFile(name string) error {
	if err := s.files.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) recordPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) segmentPath(id string, index int) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s.%d.webm", id, index))
}

func validID(id string) error {
	if !idPattern.MatchString(id) {
		return ErrInvalidID
	}
	return nil
}
