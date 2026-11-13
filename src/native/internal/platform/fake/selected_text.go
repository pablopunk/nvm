package fake

import (
	"fmt"
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpReadAccessibleSelection is the operation name accepted by SelectedText.FailNext for the accessibility read.
const OpReadAccessibleSelection = "ReadAccessibleSelection"

// OpCopySelection is the operation name accepted by SelectedText.FailNext.
const OpCopySelection = "CopySelection"

// SelectedText is a platform.SelectedText over the fake clipboard. Select sets the text the fake app has
// selected; a copy gesture puts that text on the clipboard, as a real app does, and sends nothing when no text is
// selected. Requests lists the expected app IDs of every gesture, in order.
//
//	sel := set.SelectedText
//	sel.Select("this are selected text")
//	_ = sel.CopySelection("")   // the clipboard now holds the selection
type SelectedText struct {
	mu        sync.Mutex
	clip      *Clipboard
	selection string
	requests  []string
	faults    faultSet

	accessible      string
	accessibleReads int
	modifiersHeld   bool
}

var (
	_ platform.SelectedText        = (*SelectedText)(nil)
	_ platform.AccessibleSelection = (*SelectedText)(nil)
)

// NewSelectedText returns a selection that copies onto clip.
func NewSelectedText(clip *Clipboard) *SelectedText {
	return &SelectedText{clip: clip}
}

// FailNext makes the next copy gesture return err without recording it.
func (s *SelectedText) FailNext(op string, err error) {
	s.faults.failNext(op, err)
}

// Select sets the text the fake app has selected. An empty string selects nothing.
func (s *SelectedText) Select(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selection = text
}

// CopySelection records the request and, when text is selected, puts it on the clipboard.
func (s *SelectedText) CopySelection(expectedAppID string) error {
	if err := s.faults.take(OpCopySelection); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, expectedAppID)
	if s.modifiersHeld {
		return fmt.Errorf("the shortcut's modifier keys were still held: %w", platform.ErrConflict)
	}
	if s.selection != "" {
		return s.clip.WriteText(s.selection)
	}
	return nil
}

// SelectAccessible sets the text the accessibility API reports. An app that does not expose its selection reports "".
func (s *SelectedText) SelectAccessible(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessible = text
}

// HoldModifiers makes every copy gesture fail as the real port does when the shortcut's modifier keys never come up.
func (s *SelectedText) HoldModifiers(held bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modifiersHeld = held
}

// ReadAccessibleSelection returns the accessible selection, counting the read.
func (s *SelectedText) ReadAccessibleSelection() (string, error) {
	if err := s.faults.take(OpReadAccessibleSelection); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessibleReads++
	return s.accessible, nil
}

// AccessibleReads is how many times the accessibility API was asked.
func (s *SelectedText) AccessibleReads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accessibleReads
}

// Requests returns a copy of the expected app IDs of every copy gesture, in order.
func (s *SelectedText) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}
