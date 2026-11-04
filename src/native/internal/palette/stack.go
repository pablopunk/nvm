package palette

// SetStack replaces the navigation stack without leaving the views on it. The host uses it to start from a
// given state, and the goldens use it to render one.
func (p *Palette) SetStack(views ...*View) {
	p.stack = append([]*View(nil), views...)
	p.shownView = nil
}

func (p *Palette) cur() *View { return p.stack[len(p.stack)-1] }

// Show shows the palette with every search reset, on the root or on the chat page that survived the last hide, then
// runs OnShow.
func (p *Palette) Show() {
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
	p.actionsOpen = false
	p.shownView = nil
	p.resetRootHistory()
	p.leaveUnsurvivingPages()
	p.syncPresentation()
	p.search()
	if p.opts.OnShow != nil {
		p.opts.OnShow()
	}
	p.focusInput()
	p.win.Show()
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
	p.Show()
	p.push(v)
}

// Hide closes the palette window and runs OnHide. The stack stays as it is until the next Show.
func (p *Palette) Hide() {
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
	p.win.Update(func() {
		keep := p.chosen
		p.invalidateRows()
		p.syncPresentation()
		p.search()
		p.choose(keep)
	})
}

// RefreshTop runs the search again when the top view satisfies pred and the palette is shown. It is safe from any
// goroutine, so state that changes in the background redraws an open view without the user reopening it.
func (p *Palette) RefreshTop(pred func(*View) bool) {
	p.win.Update(func() {
		if !p.win.IsVisible() || !pred(p.cur()) {
			return
		}
		keep := p.chosen
		p.invalidateRows()
		p.search()
		p.choose(keep)
	})
}

// ReloadRows forgets the rows of every view and searches again, from the UI thread: the host calls it when
// the data behind the root changed (applications, extensions).
func (p *Palette) ReloadRows() {
	p.invalidateRows()
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
	covered := p.cur()
	p.stack = append(p.stack, v)
	p.pushes++
	p.query = ""
	p.actionsOpen = false
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
	for len(p.stack) > 1 {
		p.Pop()
	}
}

func (p *Palette) Pop() {
	if len(p.stack) < 2 {
		return
	}
	p.leave(p.cur())
	p.stack = p.stack[:len(p.stack)-1]
	p.query = ""
	p.actionsOpen = false
	p.syncActive(nil)
	p.syncPresentation()
	p.search()
	p.focusInput()
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
