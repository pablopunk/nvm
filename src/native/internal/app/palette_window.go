package app

import (
	"log"

	"github.com/egoist/mygo"
)

// The window: where the palette appears and how the hotkey and the tray open it.

func (p *Palette) show() {
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	w, h := p.panel.Size()
	p.panel.SetPosition(d.WorkArea.X+(d.WorkArea.Width-w)/2, d.WorkArea.Y+(d.WorkArea.Height-h)/3)
	p.phIndex++
	for len(p.stack) > 1 {
		p.leave(p.cur())
		p.stack = p.stack[:len(p.stack)-1]
	}
	p.query = ""
	p.actionsOpen = false
	p.shownView = nil
	p.search()
	go func() {
		apps, err := mygo.Shell.InstalledApps()
		if err != nil {
			log.Println(err)
			return
		}
		p.panel.Update(func() {
			p.apps = apps
			p.invalidateRows()
			p.shownView = nil
			p.search()
		})
	}()
	p.input.Focus()
	p.panel.Show()
}

func (p *Palette) toggle() {
	if p.panel.IsVisible() {
		p.panel.Hide()
	} else {
		p.show()
	}
}

// open shows the palette directly in a sub-view (from the tray or a hotkey).
func (p *Palette) open(v *View) {
	p.show()
	p.Push(v)
}
