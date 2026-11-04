package fake

import (
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// defaultDisplay is the single 1440x900 primary display used when a test does not set displays.
var defaultDisplay = platform.Display{
	ID:          1,
	Label:       "Fixture display",
	Bounds:      platform.Rect{Width: 1440, Height: 900},
	WorkArea:    platform.Rect{Y: 25, Width: 1440, Height: 875},
	ScaleFactor: 2,
	Primary:     true,
}

// Screens is a platform.Screens with settable displays and pointer position.
//
//	screens := fake.NewScreens()
//	screens.SetCursor(platform.Point{X: 2000, Y: 100}) // on a second display, if one is set
type Screens struct {
	mu       sync.Mutex
	displays []platform.Display
	cursor   platform.Point
}

var _ platform.Screens = (*Screens)(nil)

// NewScreens returns a fake with one primary display and the pointer at its center.
func NewScreens() *Screens {
	return &Screens{
		displays: []platform.Display{defaultDisplay},
		cursor:   platform.Point{X: 720, Y: 450},
	}
}

// SetDisplays replaces the monitors.
func (s *Screens) SetDisplays(displays ...platform.Display) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.displays = slices.Clone(displays)
}

// SetCursor moves the pointer.
func (s *Screens) SetCursor(p platform.Point) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursor = p
}

// Displays returns a copy of the monitors.
func (s *Screens) Displays() []platform.Display {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.displays)
}

// CursorPoint returns the pointer position.
func (s *Screens) CursorPoint() platform.Point {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursor
}

// NearestDisplay returns the display whose bounds contain p, else the primary one, else the first.
// It returns the zero Display when there are no displays.
func (s *Screens) NearestDisplay(p platform.Point) platform.Display {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, display := range s.displays {
		if display.Bounds.Contains(p) {
			return display
		}
	}
	for _, display := range s.displays {
		if display.Primary {
			return display
		}
	}
	if len(s.displays) == 0 {
		return platform.Display{}
	}
	return s.displays[0]
}
