package extwindow

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Perform runs a declarative window action of extension extID: its op names the operation and its Window names the
// window. Errors say why the window could not do what was asked; the caller shows them.
func (m *Manager) Perform(extID string, action api.Action) error {
	var ref api.WindowRef
	if action.Window != nil {
		ref = *action.Window
	}
	switch action.Op {
	case "create":
		if ref.View == nil {
			return fmt.Errorf("a window needs a view to create: %w", ErrNoPage)
		}
		return m.Create(extID, *ref.View, optionsOf(ref))
	case "show":
		return m.Show(extID, ref)
	case "hide":
		return m.Hide(extID, ref)
	case "toggle":
		return m.Toggle(extID, ref)
	case "close":
		return m.Close(extID, ref)
	}
	return fmt.Errorf("window operation %q is not known", action.Op)
}

// Show brings an open window forward.
func (m *Manager) Show(extID string, ref api.WindowRef) error {
	w := m.find(extID, ref)
	if w == nil {
		return ErrNotOpen
	}
	w.surface.Show()
	return nil
}

// Hide hides an open window; it stays open and keeps its page.
func (m *Manager) Hide(extID string, ref api.WindowRef) error {
	w := m.find(extID, ref)
	if w == nil {
		return ErrNotOpen
	}
	w.surface.Hide()
	return nil
}

// Close closes an open window. Its record goes when the native window reports that it closed.
func (m *Manager) Close(extID string, ref api.WindowRef) error {
	w := m.find(extID, ref)
	if w == nil {
		return ErrNotOpen
	}
	w.surface.Close()
	return nil
}

// Toggle hides a visible window and shows a hidden one. A window that is not open is created when the ref carries a
// view. A window that was just hidden by losing focus stays hidden, because the click that took the focus is the
// one asking to close it. A ref with a view or options updates the window before it shows.
func (m *Manager) Toggle(extID string, ref api.WindowRef) error {
	w := m.find(extID, ref)
	if w == nil {
		if ref.View == nil {
			return ErrNotOpen
		}
		return m.Create(extID, *ref.View, optionsOf(ref))
	}
	if w.surface.IsVisible() {
		w.surface.Hide()
		return nil
	}
	if m.now().Sub(w.hiddenByBlurAt) < blurToggleGrace {
		return nil
	}
	if ref.View != nil || ref.Options != nil {
		m.refresh(w, viewOf(w, ref), optionsOfWindow(w, ref))
	}
	w.surface.Show()
	return nil
}

func (m *Manager) find(extID string, ref api.WindowRef) *window {
	id := idOf(ref)
	if id == "" {
		return nil
	}
	return m.windows[recordKey(extID, id)]
}

func idOf(ref api.WindowRef) string {
	switch {
	case ref.ID != "":
		return ref.ID
	case ref.View != nil:
		return WindowID(*ref.View, optionsOf(ref))
	}
	return ""
}

func optionsOf(ref api.WindowRef) api.WindowOptions {
	if ref.Options == nil {
		return api.WindowOptions{}
	}
	return *ref.Options
}

func viewOf(w *window, ref api.WindowRef) api.View {
	if ref.View == nil {
		return w.view
	}
	return *ref.View
}

func optionsOfWindow(w *window, ref api.WindowRef) api.WindowOptions {
	if ref.Options == nil {
		return w.options
	}
	return *ref.Options
}
