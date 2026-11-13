package diag

import (
	"crypto/rand"
	"sync"
)

// ReferencePrefix starts every support reference, so a reference is recognizable in a message or a log.
const ReferencePrefix = "NVM-"

const (
	// referenceAlphabet has 32 symbols, a power of two, so a random byte masked to five bits is uniform.
	// It leaves out 0, 1, I and O, which people confuse when they read a reference aloud.
	referenceAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	referenceLength   = 5
)

// referenceSource hands out references and never repeats one within the process. The draw function is
// injected so the collision rule can be tested with a scripted sequence.
type referenceSource struct {
	mu     sync.Mutex
	draw   func() string
	issued map[string]struct{}
}

var references = newReferenceSource(drawReference)

func newReferenceSource(draw func() string) *referenceSource {
	return &referenceSource{draw: draw, issued: map[string]struct{}{}}
}

// next draws until it finds a reference this process has not issued yet.
func (s *referenceSource) next() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		ref := s.draw()
		if _, taken := s.issued[ref]; !taken {
			s.issued[ref] = struct{}{}
			return ref
		}
	}
}

// drawReference returns a random reference such as NVM-7F3K2 from the operating system's generator.
func drawReference() string {
	var random [referenceLength]byte
	// crypto/rand.Read cannot fail on supported platforms; since Go 1.24 it crashes instead of returning
	// an error, so there is no failure path to handle here.
	_, _ = rand.Read(random[:])
	out := make([]byte, 0, len(ReferencePrefix)+referenceLength)
	out = append(out, ReferencePrefix...)
	for _, b := range random {
		out = append(out, referenceAlphabet[b&31])
	}
	return string(out)
}

// NewReference returns a new support reference such as NVM-7F3K2. References are random, never repeat
// within the process, and are the only identifier a user needs to copy when reporting a problem.
func NewReference() string {
	return references.next()
}
