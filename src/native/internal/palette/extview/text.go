package extview

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/design"
)

func drawMarkdown(c *ui.Context, src string, md *markdown.Options) {
	markdown.View(c, src, md)
}

func drawPlainText(c *ui.Context, text string) {
	t := design.Tokens(c)
	ui.Text(c, text).FontSize(11).TextColor(t.Primary)
}
