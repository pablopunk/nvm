package fake

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpPasteIntoFrontmost is the operation name accepted by Paster.FailNext.
const OpPasteIntoFrontmost = "PasteIntoFrontmost"

// OpUndoInFrontmost is the operation name accepted by Paster.FailNext.
const OpUndoInFrontmost = "UndoInFrontmost"

// Paster is a platform.Paster that records every paste request instead of sending keys. Tests read the
// requests to assert which app was targeted, and FailNext simulates the OS refusing the gesture.
//
//	paster := fake.NewPaster()
//	_ = paster.PasteIntoFrontmost("com.apple.TextEdit")
//	paster.Requests() // []string{"com.apple.TextEdit"}
type Paster struct {
	mu        sync.Mutex
	requests  []string
	undos     []string
	typed     []string
	frontmost string
	faults    faultSet
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

func (p *Paster) PasteIntoFrontmostContext(ctx context.Context, expectedAppID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.PasteIntoFrontmost(expectedAppID)
}

// UndoInFrontmost records the undo request and returns nil, or the injected fault.
func (p *Paster) UndoInFrontmost(expectedAppID string) error {
	if err := p.faults.take(OpUndoInFrontmost); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.undos = append(p.undos, expectedAppID)
	return nil
}

func (p *Paster) UndoInFrontmostContext(ctx context.Context, expectedAppID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.UndoInFrontmost(expectedAppID)
}

// Undos returns a copy of the expected app IDs of every recorded undo, in order.
func (p *Paster) Undos() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.undos)
}

// Requests returns a copy of the expected app IDs of every recorded paste, in order.
func (p *Paster) Requests() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.requests)
}

// SetFrontmost sets the identity FrontmostAppID reports.
func (p *Paster) SetFrontmost(appID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.frontmost = appID
}

// FrontmostAppID returns the identity set with SetFrontmost, or the injected fault.
func (p *Paster) FrontmostAppID() (string, error) {
	if err := p.faults.take("FrontmostAppID"); err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.frontmost, nil
}

// OpTypeIntoFrontmost is the operation name accepted by Paster.FailNext.
const OpTypeIntoFrontmost = "TypeIntoFrontmost"

// TypeIntoFrontmost records the typed text and returns nil, or the injected fault.
func (p *Paster) TypeIntoFrontmost(_, text string, _ time.Duration) error {
	if err := p.faults.take(OpTypeIntoFrontmost); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.typed = append(p.typed, text)
	return nil
}

func (p *Paster) TypeIntoFrontmostContext(ctx context.Context, expectedAppID, text string, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.TypeIntoFrontmost(expectedAppID, text, delay)
}

// Typed returns a copy of every text the paster was asked to type, in order.
func (p *Paster) Typed() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.typed)
}

var _ platform.Typist = (*Paster)(nil)
var _ platform.ContextPaster = (*Paster)(nil)
var _ platform.ContextTypist = (*Paster)(nil)
