package clipboard

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Garbage collection limits. An image younger than GCGrace is kept even when unreferenced, because a copy
// may be in flight; a sweep stops after GCDeadline or GCMaxDeletes, whichever comes first.
const (
	GCGrace      = 10 * time.Minute
	GCDeadline   = 15 * time.Second
	GCMaxDeletes = 500
	gcName       = "clipboard.images.gc"
)

// Sweep statuses and reasons. A skipped sweep deleted nothing.
const (
	GCSwept   = "swept"
	GCSkipped = "skipped"
)

// GCRefs is what a sweep needs to know about references. Trusted is false when the live history could not be
// trusted, and then nothing is deleted. ImageNames lists every name referenced by the live history and the
// saved snapshot, and fails when the saved snapshot cannot be read. HasImage is asked again right before
// each delete, so an image referenced during the sweep is kept.
type GCRefs interface {
	Trusted() bool
	ImageNames() (map[string]bool, error)
	HasImage(name string) bool
}

// GCResult reports one sweep.
type GCResult struct {
	Status  string
	Reason  string
	Deleted int
}

// gcCall is one sweep in progress. Callers that arrive while it runs wait for it and share its result.
type gcCall struct {
	done   chan struct{}
	result GCResult
}

// GC deletes stored images that no entry references and that are older than GCGrace. Only regular files whose
// names the store creates are candidates; symbolic links and other entries are never followed or deleted.
// Concurrent calls share one sweep. A panic in the sweep is recovered and reported as a skipped sweep.
func (s *ImageStore) GC(refs GCRefs) GCResult {
	s.mu.Lock()
	if call := s.inflight; call != nil {
		s.joined++
		s.mu.Unlock()
		<-call.done
		return call.result
	}
	call := &gcCall{done: make(chan struct{})}
	s.inflight = call
	s.mu.Unlock()

	call.result = s.sweepGuarded(refs)

	s.mu.Lock()
	s.inflight = nil
	s.mu.Unlock()
	close(call.done)
	return call.result
}

// sweepGuarded runs one sweep under safego, logging a failure without its content.
func (s *ImageStore) sweepGuarded(refs GCRefs) GCResult {
	var result GCResult
	err := safego.Run(gcName, func(context.Context) error {
		r, err := s.sweep(refs)
		result = r
		return err
	}, safego.WithClock(s.deps.Clock), safego.WithReporter(silentReporter{}))
	if err != nil {
		s.deps.Log.Warn("clipboard.images.sweep.failed", logx.F{"error": err.Error()})
		return GCResult{Status: GCSkipped, Reason: "error"}
	}
	return result
}

// silentReporter keeps safego from printing a failure; sweepGuarded logs it through logx instead.
type silentReporter struct{}

// Report discards the failure.
func (silentReporter) Report(safego.Failure) string { return "" }

// sweep does the work of one GC call. It returns an error only for a failure of the folder listing; the
// caller reports it.
func (s *ImageStore) sweep(refs GCRefs) (GCResult, error) {
	if !refs.Trusted() {
		return GCResult{Status: GCSkipped, Reason: "untrusted-state"}, nil
	}
	entries, err := s.deps.Files.ReadDir(s.deps.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return GCResult{Status: GCSwept, Reason: "no-candidates"}, nil
	}
	if err != nil {
		return GCResult{}, err
	}
	candidates := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && imageNamePattern.MatchString(name) && !refs.HasImage(name) {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) == 0 {
		return GCResult{Status: GCSwept, Reason: "no-candidates"}, nil
	}
	referenced, err := refs.ImageNames()
	if err != nil {
		return GCResult{Status: GCSkipped, Reason: "persisted-unreadable"}, nil
	}
	return s.deleteOrphans(refs, candidates, referenced), nil
}

// deleteOrphans deletes the candidates that are still unreferenced, within the deadline and delete cap.
func (s *ImageStore) deleteOrphans(refs GCRefs, candidates []string, referenced map[string]bool) GCResult {
	deadline := s.deps.Clock.Now().Add(GCDeadline)
	deleted := 0
	for _, name := range candidates {
		if referenced[name] {
			continue
		}
		if deleted >= GCMaxDeletes || s.deps.Clock.Now().After(deadline) {
			break
		}
		removed, err := s.deleteIfOrphaned(refs, name)
		if err != nil {
			s.deps.Log.Warn("clipboard.images.sweep.unlinkFailed", logx.F{"name": name, "error": err.Error()})
			continue
		}
		if removed {
			deleted++
		}
	}
	return GCResult{Status: GCSwept, Deleted: deleted}
}

// deleteIfOrphaned removes one image when it is an old regular file that no entry references at this moment.
// It reports whether it removed the file.
func (s *ImageStore) deleteIfOrphaned(refs GCRefs, name string) (bool, error) {
	path := filepath.Join(s.deps.Dir, name)
	info, err := s.deps.Files.Lstat(path)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || s.deps.Clock.Now().Sub(info.ModTime()) < GCGrace {
		return false, nil
	}
	if refs.HasImage(name) {
		return false, nil
	}
	if err := s.deps.Files.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}

// joinedCount reports how many calls shared another sweep. Tests use it to wait for a caller to join.
func (s *ImageStore) joinedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.joined
}
