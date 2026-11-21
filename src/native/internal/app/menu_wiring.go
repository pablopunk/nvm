package app

import (
	"context"
	"time"

	menusbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/menus"
	"github.com/pablopunk/nvm/native/internal/ext/menushost"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func (p *App) attachMenus(helper platform.Helper) {
	source := real.NewCurrentAppMenus(helper)
	if source == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.menuCancel = cancel
	p.menus = menushost.New(ctx, source, 2*time.Second, p.menuChanged)
}

func (p *App) menuPort() services.MenuPort {
	if p.menus == nil {
		return nil
	}
	return p.menus
}

func (p *App) menuChanged() {
	p.onUI(func() {
		if !p.quitting && p.Palette != nil {
			p.NotifyTopDataChanged(menusbuiltin.ID)
		}
	})
}

func (p *App) captureMenusOnShow() {
	pid := p.captureWindowFocus()
	if p.menus != nil {
		if pid > 0 {
			p.menus.Capture(pid)
		} else {
			p.menus.Dismiss()
		}
	}
}
