package fake

import (
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpPasteIntoFrontmost is the operation name accepted by Paster.FailNext.
const OpPasteIntoFrontmost = "PasteIntoFrontmost"

// Paster is a platform.Paster that records every paste request instead of sending keys. Tests read the
// requests to assert which app was targeted, and FailNext simulates the OS refusing the gesture.
//
//	paster := fake.NewPaster()
//	_ = paster.PasteIntoFrontmost("com.apple.TextEdit")
//	paster.Requests() // []string{"com.apple.TextEdit"}
type Paster struct {
	mu       sync.Mutex
	requests []string
	faults   faultSet
}

var _ platform.Paster = (*Paster)(nil)

// NewPaster returns a paster with no recorded requests.
func NewPaster() *Paster {
	return &Paster{}
}

// FailNext makes the next paste request return err without recording it.
func (p *Paster) FailNext(op string, err error) {
	p.faults.failNext(op, err)
}

// PasteIntoFrontmost records the request and returns nil, or the injected fault.
func (p *Paster) PasteIntoFrontmost(expectedAppID string) error {
	if err := p.faults.take(OpPasteIntoFrontmost); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, expectedAppID)
	return nil
}

// Requests returns a copy of the expected app IDs of every recorded paste, in order.
func (p *Paster) Requests() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.requests)
}
