// Package recordings keeps the audio of a dictation until its transcript is in, so a dictation that failed or was cut
// off can be transcribed again from the history. Each dictation is a JSON record and one WebM file per minute of
// speech, written under the data folder; the record is written last, so a save that stops halfway is never listed.
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
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// MaxRecordings is how many saved dictations are kept; the oldest go first.
const MaxRecordings = 25

// MaxTotalBytes bounds the audio on disk. Once the saved dictations together pass it, the oldest go first; the newest
// recording is always kept, since it is the one the user is most likely to retry.
const MaxTotalBytes int64 = 128 << 20

// maxSegments is the longest recording the store takes: 120 one-minute segments, as the Electron app allows.
const maxSegments = 120

// ErrInvalidID is returned for a recording name that is not a dictation's.
var ErrInvalidID = errors.New("dictation recordings: invalid recording id")

// ErrNotFound is returned when no saved recording has the id.
var ErrNotFound = errors.New("dictation recordings: recording not found")

// ErrSegmentCount is returned for a recording with no segments or more than maxSegments.
var ErrSegmentCount = errors.New("dictation recordings: a recording needs between one and 120 segments")

var idPattern = regexp.MustCompile(`^dictation-[A-Za-z0-9-]{1,80}$`)

// Recording describes one saved dictation.
type Recording struct {
	ID           string `json:"id"`
	CreatedAt    int64  `json:"createdAt"`
	SegmentCount int    `json:"segmentCount"`
	// Failure says why the last transcription failed, in the words the user reads. Empty for a dictation that was cut
	// off (the app closed while it was on its way) and never got an answer.
	Failure string `json:"failure,omitempty"`
}

// Store reads and writes the saved dictations in dir. It is safe to use from several goroutines when the file port is.
type Store struct {
	files platform.FS
	dir   string
	now   func() time.Time
	// maxBytes is the audio bound; tests lower it.
	maxBytes int64
}

// New returns a store over the folder dir. now stamps each recording, so tests control the order.
func New(files platform.FS, dir string, now func() time.Time) *Store {
	return &Store{files: files, dir: dir, now: now, maxBytes: MaxTotalBytes}
}

// Save writes the segments of the dictation id and then its record, and drops the oldest recordings past the limit.
func (s *Store) Save(id string, segments [][]byte) error {
	if err := validID(id); err != nil {
		return err
	}
	if len(segments) == 0 || len(segments) > maxSegments {
		return ErrSegmentCount
	}
	if err := s.files.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	for index, audio := range segments {
		if err := s.files.WriteFile(s.segmentPath(id, index), audio, 0o600); err != nil {
			return err
		}
	}
	record, err := json.Marshal(Recording{ID: id, CreatedAt: s.now().UnixMilli(), SegmentCount: len(segments)})
	if err != nil {
		return err
	}
	if err := s.files.WriteFile(s.recordPath(id), record, 0o600); err != nil {
		return err
	}
	return s.prune()
}

// MarkFailed records why the transcription of the saved dictation id failed, so the history can say so after a restart.
func (s *Store) MarkFailed(id, failure string) error {
	recording, err := s.readRecord(id)
	if err != nil {
		return err
	}
	recording.Failure = failure
	record, err := json.Marshal(recording)
	if err != nil {
		return err
	}
	return s.files.WriteFile(s.recordPath(id), record, 0o600)
}

// List returns the saved recordings, newest first. A folder entry that is not a complete record is skipped.
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
		if recording, err := s.readRecord(id); err == nil {
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

// Remove deletes the saved dictation id. A recording that is already gone is not an error.
func (s *Store) Remove(id string) error {
	if err := validID(id); err != nil {
		return err
	}
	recording, err := s.readRecord(id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	for index := range recording.SegmentCount {
		if err := s.removeFile(s.segmentPath(id, index)); err != nil {
			return err
		}
	}
	return s.removeFile(s.recordPath(id))
}

// prune removes the oldest recordings beyond MaxRecordings or MaxTotalBytes, and the segments of saves that stopped
// before their record was written. Saves are serial, so none is in flight when prune runs.
func (s *Store) prune() error {
	recordings, err := s.List()
	if err != nil {
		return err
	}
	var total int64
	for index, recording := range recordings {
		total += s.audioBytes(recording)
		if index > 0 && (index >= MaxRecordings || total > s.maxBytes) {
			if err := s.Remove(recording.ID); err != nil {
				return err
			}
		}
	}
	return s.removeOrphanSegments()
}

// audioBytes is the size of the segments of recording. A segment that cannot be read counts as empty.
func (s *Store) audioBytes(recording Recording) int64 {
	var total int64
	for index := range recording.SegmentCount {
		if info, err := s.files.Stat(s.segmentPath(recording.ID, index)); err == nil {
			total += info.Size()
		}
	}
	return total
}

var segmentPattern = regexp.MustCompile(`^(dictation-[A-Za-z0-9-]{1,80})\.\d+\.webm$`)

// removeOrphanSegments deletes the segment files that have no record, left by a save that stopped halfway.
func (s *Store) removeOrphanSegments() error {
	entries, err := s.files.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		match := segmentPattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		if _, err := s.readRecord(match[1]); errors.Is(err, ErrNotFound) {
			if err := s.removeFile(filepath.Join(s.dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// readRecord reads and checks the record of id.
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
	if err := json.Unmarshal(data, &recording); err != nil {
		return Recording{}, err
	}
	if recording.ID != id || recording.SegmentCount < 1 || recording.SegmentCount > maxSegments {
		return Recording{}, ErrNotFound
	}
	return recording, nil
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
