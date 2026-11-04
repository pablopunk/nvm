package fake

import (
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

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
}

var _ platform.SelectedText = (*SelectedText)(nil)

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
	if s.selection != "" {
		return s.clip.WriteText(s.selection)
	}
	return nil
}

// Requests returns a copy of the expected app IDs of every copy gesture, in order.
func (s *SelectedText) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}
