package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"testing"
)

func TestReplyCodeKeepsDefaultInteriorPadding(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "scroll", true: "wrap"}[wrap], func(t *testing.T) {
			options := markdown.Options{WrapCode: wrap}
			tester := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).FillWidth().Children(func() { replyMarkdown(c, "```txt\npadding_probe\n```", false, &options) })
			}, 600, 240)
			tester.Frame()
			block, ok := tester.Find("Code, txt")
			if !ok {
				t.Fatal("reply has no labeled code block")
			}
			content, ok := tester.Find("padding_probe\n")
			if !ok {
				content, ok = tester.Find("padding_probe")
			}
			if !ok {
				t.Fatal("reply has no rendered code content")
			}
			if content.X-block.X < 4 || block.X+block.W-content.X-content.W < 4 || block.Y+block.H-content.Y-content.H < 4 {
				t.Fatalf("code touches its container edge: block=%+v content=%+v", block, content)
			}
		})
	}
}
