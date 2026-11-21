package extwindow

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fakeOpener records the windows it opens. Each surface is a fakeSurface the test drives.
type fakeOpener struct {
	surfaces []*fakeSurface
}

func (o *fakeOpener) Open(spec Spec) Surface {
	s := &fakeSurface{spec: spec}
	o.surfaces = append(o.surfaces, s)
	return s
}

// fakeSurface is a window that keeps what it was asked for. blur and move play the native events of the window.
type fakeSurface struct {
	spec      Spec
	root      *palette.View
	visible   bool
	shows     int
	frame     Frame
	closedFns []func()
	blurFns   []func()
	moveFns   []func()
}

func (s *fakeSurface) Show()                      { s.visible = true; s.shows++ }
func (s *fakeSurface) Hide()                      { s.visible = false }
func (s *fakeSurface) IsVisible() bool            { return s.visible }
func (s *fakeSurface) SetRoot(root *palette.View) { s.root = root }
func (s *fakeSurface) Reconfigure(spec Spec)      { s.spec = spec }
func (s *fakeSurface) Host() search.Host          { return nil }
func (s *fakeSurface) Frame() Frame               { return s.frame }
func (s *fakeSurface) OnFrameChanged(fn func())   { s.moveFns = append(s.moveFns, fn) }
func (s *fakeSurface) OnBlur(fn func())           { s.blurFns = append(s.blurFns, fn) }
func (s *fakeSurface) OnClosed(fn func())         { s.closedFns = append(s.closedFns, fn) }

func (s *fakeSurface) Close() {
	s.visible = false
	for _, fn := range s.closedFns {
		fn()
	}
}

func (s *fakeSurface) blur() {
	for _, fn := range s.blurFns {
		fn()
	}
}

func (s *fakeSurface) move(frame Frame) {
	s.frame = frame
	for _, fn := range s.moveFns {
		fn()
	}
}

// fakeSaver keeps the saved state in memory, as the store collection does on disk.
type fakeSaver struct {
	state Saved
}

func (f *fakeSaver) Load() (Saved, error) { return f.state, nil }

func (f *fakeSaver) Update(change func(*Saved)) error {
	change(&f.state)
	return nil
}

// testPage draws a view as a page with its title, which is all the manager needs of a page.
func testPage(_ string, view api.View, _ search.Host) *palette.View {
	return &palette.View{Title: view.Title}
}

// newTestManager builds a manager over fakes. OnUI runs its callbacks in place, so a restore finishes in the call.
func newTestManager(opener *fakeOpener, saver Saver, clock *fake.Clock) *Manager {
	return New(Deps{Open: opener, Page: testPage, Saved: saver, Clock: clock})
}
