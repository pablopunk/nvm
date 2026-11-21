package palette

import "github.com/pablopunk/nvm/native/internal/diag/analytics"

// NavigationCallback receives the category of a newly presented palette page.
type NavigationCallback func(analytics.PageView)

type navigationState struct {
	generation uint64
	reported   uint64
}

type navigationFrame struct {
	view       *View
	generation uint64
	shows      int
}

func (s *navigationState) advance() { s.generation++ }

func (p *Palette) captureNavigationFrame() navigationFrame {
	return navigationFrame{view: p.cur(), generation: p.navigation.generation, shows: p.shows}
}

func (p *Palette) reportNavigationFrame(frame navigationFrame) {
	callback := p.opts.OnNavigation
	if callback == nil || p.win == nil || !p.win.IsVisible() || p.revealWaiting ||
		frame.view != p.cur() || frame.generation != p.navigation.generation || frame.shows != p.shows ||
		frame.view.navigationPlaceholder || p.navigation.reported == frame.generation {
		return
	}
	category := analytics.PageViewPalette
	if len(p.stack) > 1 {
		category = analytics.PageViewExtension
	}
	if frame.view.Category != "" {
		category = frame.view.Category
	}
	switch category {
	case analytics.PageViewPalette, analytics.PageViewExtension, analytics.PageViewAIChat:
		p.navigation.reported = frame.generation
		callback(category)
	}
}
