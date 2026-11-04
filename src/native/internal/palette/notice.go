package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// Notice is a banner between the search field and the results: what is wrong and the one action that fixes
// it. The host sets it while the problem lasts and clears it when the problem is gone. Action runs when the
// button is pressed.
type Notice struct {
	Icon        string
	Title       string
	Detail      string
	ActionLabel string
	Alert       bool
	Action      func()
}

// SetNotice shows n above the results, or clears the banner when n is nil.
func (p *Palette) SetNotice(n *Notice) { p.notice = n }

// noticeRow draws the banner, when there is one, and runs its action when the button is pressed.
func (p *Palette) noticeRow(c *ui.Context) {
	n := p.notice
	if n == nil {
		return
	}
	data := design.NoticeData{Icon: n.Icon, Title: n.Title, Detail: n.Detail, ActionLabel: n.ActionLabel, Alert: n.Alert}
	if design.Notice(c, data) && n.Action != nil {
		n.Action()
	}
}
