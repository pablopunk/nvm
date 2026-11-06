package app

import "github.com/pablopunk/nvm/native/internal/diag/analytics"

func (p *App) recordPaletteNavigation(view analytics.PageView) {
	p.analytics.PageViewed(view)
}
