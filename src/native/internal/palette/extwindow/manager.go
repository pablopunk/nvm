package extwindow

import (
	"errors"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ErrNotOpen answers a show, hide, toggle or close of a window the extension does not have open.
var ErrNotOpen = errors.New("window is not open")

// ErrNoPage answers a window whose view cannot be drawn, such as while the extension host is not running.
var ErrNoPage = errors.New("the extension's window is not available")

// blurToggleGrace is how soon after a hide-on-blur a toggle still reads as the click that caused the blur: the
// window stays closed rather than reopening.
const blurToggleGrace = 300 * time.Millisecond

// Deps are the services of the manager. Page builds the palette page a window shows; Saved keeps the persistent
// windows and remembered frames, and nil keeps them for the session only; Clock times the blur grace; OnUI runs a
// callback on the UI thread.
type Deps struct {
	Open      Opener
	Page      func(extID string, view api.View, host search.Host) *palette.View
	PageBound func(extID string, identity any, view api.View, host search.Host) *palette.View
	Saved     Saver
	Clock     platform.Clock
	OnUI      func(fn func())
}

// Manager owns the windows of every extension. It runs on the UI thread; RestorePersisted hands its results back
// through OnUI.
type Manager struct {
	deps     Deps
	windows  map[string]*window
	quitting bool
}

// window is one open window of an extension. view and options are what it shows now.
type window struct {
	extID          string
	authority      Authority
	id             string
	view           api.View
	options        api.WindowOptions
	surface        Surface
	hiddenByBlurAt time.Time
}

// New returns a manager with no window open.
func New(deps Deps) *Manager {
	if deps.OnUI == nil {
		deps.OnUI = func(fn func()) { fn() }
	}
	return &Manager{deps: deps, windows: map[string]*window{}}
}

// BeginQuit marks the app as quitting. Windows that close from now on keep their saved state, so the next launch
// restores them.
func (m *Manager) BeginQuit() {
	m.quitting = true
}

// Create opens a window of extension extID that shows view, or updates the window with the same id: it shows the new
// view and options, and brings the window forward.
func (m *Manager) Create(extID string, view api.View, options api.WindowOptions) error {
	return m.createBound(extID, nil, view, options)
}

func (m *Manager) createBound(extID string, authority Authority, view api.View, options api.WindowOptions) error {
	if authority != nil && !authority.Valid() {
		return ErrExpired
	}
	if err := options.Validate(); err != nil {
		return err
	}
	id := WindowID(view, options)
	if existing := m.windows[recordKey(extID, id)]; existing != nil {
		if !sameAuthority(existing.authority, authority) {
			if existing.authority == nil || existing.authority.Valid() {
				return ErrExpired
			}
			existing.surface.Close()
		} else {
			if authority != nil && !authority.Valid() {
				return ErrExpired
			}
			m.refresh(existing, view, options)
			if authority != nil && !authority.Valid() {
				return ErrExpired
			}
			existing.surface.Show()
			return nil
		}
	}
	w := &window{extID: extID, id: id, view: view, options: options, authority: authority}
	if err := m.open(w); err != nil {
		return err
	}
	if authority != nil && !authority.Valid() {
		w.surface.Close()
		return ErrExpired
	}
	m.windows[w.key()] = w
	m.persist(w)
	if authority != nil && !authority.Valid() {
		w.surface.Close()
		return ErrExpired
	}
	w.surface.Show()
	return nil
}

// refresh gives an open window its new view and options. A view that cannot be drawn leaves the page as it was.
func (m *Manager) refresh(w *window, view api.View, options api.WindowOptions) {
	if w.authority != nil && !w.authority.Valid() {
		return
	}
	root := m.page(w, view, w.surface.Host())
	if w.authority != nil && !w.authority.Valid() {
		return
	}
	w.view, w.options = view, options
	if root != nil {
		w.surface.SetRoot(root)
	}
	w.surface.Reconfigure(specFor(w.view, w.options))
	m.persist(w)
}

func (m *Manager) page(w *window, view api.View, host search.Host) *palette.View {
	if w.authority != nil && m.deps.PageBound != nil {
		return m.deps.PageBound(w.extID, w.authority.Identity(), view, host)
	}
	if m.deps.Page == nil {
		return nil
	}
	return m.deps.Page(w.extID, view, host)
}

// open makes the native window of w and gives it its page. Its callbacks keep the manager's records, the remembered
// frame and the hide on blur in step with the window. A view that cannot be drawn closes the window again.
func (m *Manager) open(w *window) error {
	spec := specFor(w.view, w.options)
	if w.options.RemembersFrame {
		spec.Saved = m.savedFrame(w)
	}
	surface := m.deps.Open.Open(spec)
	root := m.page(w, w.view, surface.Host())
	if root == nil {
		surface.Close()
		return ErrNoPage
	}
	surface.SetRoot(root)
	w.surface = surface
	surface.OnClosed(func() { m.closed(w) })
	surface.OnBlur(func() {
		if w.options.HideOnBlur {
			w.hiddenByBlurAt = m.now()
			surface.Hide()
		}
	})
	if w.options.RemembersFrame {
		surface.OnFrameChanged(func() { m.rememberFrame(w) })
	}
	return nil
}

// closed forgets a window that closed. A window that closes while the app quits keeps its saved state, so the next
// launch restores it; closing it by hand, or with a restore key the extension no longer wants, drops it.
func (m *Manager) closed(w *window) {
	if m.windows[w.key()] == w {
		delete(m.windows, w.key())
	}
	if !m.quitting && w.options.RestoreKey != "" && (w.authority == nil || w.authority.Valid()) {
		m.forgetWindow(w)
	}
}

func (m *Manager) now() time.Time {
	return m.deps.Clock.Now()
}

func (w *window) key() string {
	return recordKey(w.extID, w.id)
}

func recordKey(extID, id string) string {
	return extID + "/" + id
}

// specFor is the spec of a window that shows view with options. The title falls back to the view's title, then to the
// app's name, as the 1.x window does.
func specFor(view api.View, options api.WindowOptions) Spec {
	width, height := Size(options)
	title := options.Title
	if title == "" {
		title = view.Title
	}
	if title == "" {
		title = "Nevermind"
	}
	return Spec{
		Title:              title,
		Width:              width,
		Height:             height,
		Position:           options.Position,
		AlwaysOnTop:        options.AlwaysOnTop == nil || *options.AlwaysOnTop,
		VisibleOnAllSpaces: options.VisibleOnAllSpaces,
		TitleBarHidden:     options.TitleBar == "hidden",
		Frameless:          options.Chrome == "none",
	}
}
