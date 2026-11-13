package palette

// SetStack replaces the navigation stack without leaving the views on it. The host uses it to start from a
// given state, and the goldens use it to render one.
func (p *Palette) SetStack(views ...*View) {
	p.navigation.advance()
	p.cancelAction()
	p.resetActionMenu()
	p.stack = append([]*View(nil), views...)
	p.entryBoundary = 0
	p.shownView = nil
}

func (p *Palette) cur() *View { return p.stack[len(p.stack)-1] }

// Show shows the palette with every search reset, on the root or on the chat page that survived the last hide, then
// runs OnShow.
func (p *Palette) Show() {
	p.prepareShow()
	p.leaveUnsurvivingPages()
	p.present()
}

func (p *Palette) prepareShow() {
	p.navigation.advance()
	p.defaultEnter = nil
	p.cancelAction()
	if !p.win.IsVisible() || !p.win.IsFocused() {
		p.captureFocus()
	}
	p.shows++
	p.blurred = false
	if p.opts.Position != nil {
		w, h := p.win.Size()
		x, y := p.opts.Position(w, h)
		p.win.SetPosition(x, y)
	}
	p.phIndex++
	p.query = ""
	p.resetActionMenu()
	p.shownView = nil
	p.resetRootHistory()
	p.entryBoundary = 0
}

func (p *Palette) present() {
	p.syncPresentation()
	p.search()
	if p.opts.OnShow != nil {
		p.opts.OnShow()
	}
	p.focusInput()
	p.showPreparedWindow()
	p.syncActive(nil)
}

// Toggle is the summon shortcut's action, as 1.x reads it: a palette that is shown and has the keyboard hides. Any other
// state shows it, so a shown palette behind another window comes forward. A focus loss still within its grace counts as
// the keyboard being here, because the shortcut's own press causes that loss.
func (p *Palette) Toggle() {
	if p.win.IsVisible() && (p.blurred || p.win.IsFocused()) {
		p.Hide()
		return
	}
	p.Show()
}

// Open shows the palette with v pushed above the root, as the tray and the hotkey's sub-commands do.
func (p *Palette) Open(v *View) {
	p.prepareShow()
	p.leaveUnsurvivingPages()
	p.entryBoundary = len(p.stack)
	p.push(v)
	p.present()
}

// Hide closes the palette window and runs OnHide. The stack stays as it is until the next Show.
func (p *Palette) Hide() {
	p.cancelAction()
	p.hide()
}

// hide performs dismissal after its caller decides whether pending action completion remains valid.
func (p *Palette) hide() {
	p.defaultEnter = nil
	p.resetActionMenu()
	p.actionPending = false
	for _, v := range p.stack {
		if v.OnHide != nil {
			v.OnHide()
		}
	}
	p.win.Hide()
	p.syncActive(nil)
	p.returnFocus()
	if p.opts.OnHide != nil {
		p.opts.OnHide()
	}
}

// Refresh runs the current search again and keeps the cursor where it is. It is safe from any goroutine.
func (p *Palette) Refresh() {
	if p.win == nil {
		return
	}
	p.win.Update(func() {
		p.syncPresentation()
		p.search()
	})
}

// RefreshTop runs the search again when the top view satisfies pred and the palette is shown. It is safe from any
// goroutine, so state that changes in the background redraws an open view without the user reopening it.
func (p *Palette) RefreshTop(pred func(*View) bool) {
	if p.win == nil {
		return
	}
	p.win.Update(func() {
		if !p.win.IsVisible() || !pred(p.cur()) {
			return
		}
		p.search()
	})
}

// NotifyTopDataChanged requests a declared refresh only on the visible extension page.
func (p *Palette) NotifyTopDataChanged(extensionID string) {
	if p.win == nil {
		return
	}
	p.win.Update(func() {
		if !p.win.IsVisible() || len(p.stack) == 0 {
			return
		}
		v := p.cur()
		if v.ExtensionID == extensionID && v.OnDataChanged != nil {
			v.OnDataChanged()
		}
	})
}

// ReloadRows revalidates the visible rows from the UI thread after host data changes. Cached snapshots stay
// available for instant navigation; every search revalidates its providers.
func (p *Palette) ReloadRows() {
	p.shownView = nil
	p.search()
}

// Push opens a view above the current one. It implements search.Host, so the argument is a *View.
func (p *Palette) Push(v any) {
	view, ok := v.(*View)
	if !ok {
		panic("palette: Push wants a *View")
	}
	p.push(view)
}

func (p *Palette) push(v *View) {
	p.navigation.advance()
	if p.openingView == p.cur() {
		p.leave(p.cur())
		p.stack = p.stack[:len(p.stack)-1]
	}
	p.cancelAction()
	covered := p.cur()
	p.stack = append(p.stack, v)
	p.pushes++
	p.query = ""
	p.resetActionMenu()
	p.syncActive(covered)
	p.syncPresentation()
	p.search()
	p.focusInput()
}

// RunChosenAction runs the action the open action menu has chosen and closes the menu. A page whose text input takes
// Enter calls it, since such an input takes the key before the palette's own keys see it. It reports false when no menu
// is open.
func (p *Palette) RunChosenAction() bool {
	if !p.actionsOpen {
		return false
	}
	p.onEnter()
	return true
}

// Visible reports whether the palette window is shown.
func (p *Palette) Visible() bool { return p.win.IsVisible() }

// Presented reports whether the palette is shown and has the keyboard, so a show now re-presents it rather than coming
// from another app. OnShow runs before the window is shown, so there it tells whether the app in front is still the one
// the palette was summoned over.
func (p *Palette) Presented() bool { return p.win != nil && p.win.IsVisible() && p.win.IsFocused() }

// PopIf goes back from v when v is the page on top, and reports whether it did. It never removes the root.
func (p *Palette) PopIf(v *View) bool {
	if len(p.stack) < 2 || p.cur() != v {
		return false
	}
	p.Pop()
	return true
}

// ReplaceIf swaps the top view for v when the top view is old, and searches again without showing the palette, so an
// open page takes new items as its host changes them. It reports whether it swapped. It runs on the UI thread.
func (p *Palette) ReplaceIf(old, v *View) bool {
	if len(p.stack) < 2 || p.cur() != old {
		return false
	}
	p.leave(old)
	p.stack[len(p.stack)-1] = v
	p.search()
	return true
}

// Pop returns to the view below the current one. At the root it does nothing.
func (p *Palette) Root() {
	p.cancelAction()
	for len(p.stack) > 1 && p.entryBoundary == 0 {
		p.Pop()
	}
	if p.entryBoundary > 0 && len(p.stack) > p.entryBoundary {
		for len(p.stack) > p.entryBoundary+1 {
			p.popOne()
		}
	}
}

func (p *Palette) Pop() {
	p.cancelAction()
	if len(p.stack) < 2 {
		return
	}
	if p.entryBoundary > 0 && len(p.stack) == p.entryBoundary+1 {
		p.Hide()
		return
	}
	p.popOne()
}

// PopPage removes the top page like Pop, but never dismisses the palette at an independent entry. A host that pushed a
// loading page and now swaps it for the result calls it, since that is not the user going back.
func (p *Palette) PopPage() {
	p.cancelAction()
	if len(p.stack) < 2 {
		return
	}
	p.popOne()
}

// popOne removes a page without applying the independent-entry dismissal boundary. Root uses it to
// unwind extension navigation internally while user initiated back remains confined to its entry.
func (p *Palette) popOne() {
	p.navigation.advance()
	p.leave(p.cur())
	p.stack = p.stack[:len(p.stack)-1]
	p.query = ""
	p.resetActionMenu()
	p.syncActive(nil)
	p.syncPresentation()
	p.search()
	p.focusInput()
	if reveal := p.cur().OnReveal; reveal != nil {
		reveal()
	}
}

// leaveUnsurvivingPages pops the pages above the topmost page that survives a hide, down to the root when none does.
// It runs when the palette shows, as Electron's onShown does: Electron's hidePanes keeps the last live AI chat the same
// way and clears the rest.
func (p *Palette) leaveUnsurvivingPages() {
	for len(p.stack) > 1 && !p.cur().SurvivesHide {
		p.leave(p.cur())
		p.stack = p.stack[:len(p.stack)-1]
	}
}

// leave runs a view's OnLeave and stops its searches: nothing it returns is shown again.
func (p *Palette) leave(v *View) {
	if f := v.OnLeave; f != nil {
		f()
	}
	if v.coord != nil {
		v.coord.Cancel()
	}
}
