package extwindow

import (
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"reflect"
)

// Authority is the host-owned installation identity and its revocation state.
type Authority interface {
	Identity() any
	Valid() bool
}

// ErrExpired says the originating installation was disabled or replaced.
var ErrExpired = errors.New("extension installation changed")

func sameAuthority(a, b Authority) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	x, y := a.Identity(), b.Identity()
	if x != nil && !reflect.TypeOf(x).Comparable() {
		return false
	}
	return x == y
}

// PerformBound admits an operation for the captured installation only.
func (m *Manager) PerformBound(extID string, authority Authority, action api.Action) error {
	if authority == nil || !authority.Valid() {
		return ErrExpired
	}
	var ref api.WindowRef
	if action.Window != nil {
		ref = *action.Window
	}
	if action.Op == "create" {
		if ref.View == nil {
			return ErrNoPage
		}
		return m.createBound(extID, authority, *ref.View, optionsOf(ref))
	}
	w := m.find(extID, ref)
	if w == nil {
		if action.Op == "toggle" && ref.View != nil {
			return m.createBound(extID, authority, *ref.View, optionsOf(ref))
		}
		return ErrNotOpen
	}
	if !sameAuthority(w.authority, authority) || !authority.Valid() {
		return ErrExpired
	}
	switch action.Op {
	case "show":
		w.surface.Show()
	case "hide":
		w.surface.Hide()
	case "close":
		w.surface.Close()
	case "toggle":
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
		if !authority.Valid() {
			return ErrExpired
		}
		w.surface.Show()
	default:
		return fmt.Errorf("window operation %q is not known", action.Op)
	}
	return nil
}
