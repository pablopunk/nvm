package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// replyMarkdown shows a chat reply as Markdown. A reply still arriving streams in, revealed at an even pace; a
// finished one is parsed once. options is kept between frames, so links and code blocks stay configured.
// It lives here rather than in the design package, which may not import the Markdown renderer (layer rules).
func replyMarkdown(c *ui.Context, src string, streaming bool, options *markdown.Options) {
	if streaming {
		markdown.Stream(c, src, options)
		return
	}
	markdown.View(c, src, options)
}
