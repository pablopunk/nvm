package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ---- extension builder: the model writes a Go extension, Yaegi checks it, we install it ----

var goBlockRe = regexp.MustCompile("(?s)```(?:go|golang)?\\s*\\n(.*?)```")

func extractGo(s string) string {
	ms := goBlockRe.FindAllStringSubmatch(s, -1)
	if len(ms) == 0 {
		return ""
	}
	return ms[len(ms)-1][1]
}

func (p *Palette) builderView() *View { return p.builderViewWith("") }

// builderViewWith opens the extension builder and, given a prompt, sends it at once ("Automate ...").
func (p *Palette) builderViewWith(prompt string) *View {
	ch := &Chat{ID: newID("builder"), Updated: time.Now(), Builder: true}
	p.chats.Chats = append([]*Chat{ch}, p.chats.Chats...)
	v := p.chatView(ch)
	if prompt != "" {
		ch.Title = `Automate "` + prompt + `"`
		p.panel.Update(func() { p.sendChat(p.runs[ch.ID], prompt) })
	}
	return v
}

func (p *Palette) notice(run *chatRun, text string) {
	run.chat.Messages = append(run.chat.Messages, ChatMsg{"notice", text})
}

func (p *Palette) builderDone(run *chatRun) {
	last := run.chat.Messages[len(run.chat.Messages)-1].Content
	code := extractGo(last)
	if code == "" {
		return
	}
	ext, err := evalExtension(code)
	if err != nil {
		if run.attempts < 2 {
			run.attempts++
			p.notice(run, "Extension failed to load ("+oneLine(err.Error())+") — asking the model to fix it…")
			p.sendChat(run, "The extension failed to load with this error:\n\n"+err.Error()+"\n\nReply with the complete corrected file in one ```go block.")
			return
		}
		p.notice(run, "✗ Could not produce a working extension: "+oneLine(err.Error()))
		return
	}
	file := filepath.Join(extensionsDir(), sanitize(ext.ID)+".go")
	if os.WriteFile(file, []byte(code), 0o644) != nil {
		p.notice(run, "✗ Could not write "+file)
		return
	}
	p.reloadExtensions()
	p.notice(run, "✓ Installed “"+ext.Title+"” — "+file)
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' {
			return r
		}
		return '-'
	}, s)
}

const builderSystem = `You write extensions for Nevermind, a desktop command palette. An extension is ONE Go file interpreted at runtime.

Rules:
- Reply with a short sentence and exactly one ` + "```go" + ` code block containing the complete file.
- Keep it under 60 lines, no comments, no explanation after the code block.
- ` + "`package main`" + `, import "nvm" plus any standard-library packages. No other imports, no goroutines, no cgo.
- Define ` + "`func Extension() nvm.Extension`" + `.

API (package nvm):
  type Item struct { Title, Subtitle, Value, URL string }   // Enter copies Value, or opens URL if set
  type MenuRow struct { Title, Value string }                  // row in a tray menu; click copies Value
  type Extension struct {
      ID, Title string                                       // ID like "user.my-extension"
      Search  func(query string) []Item                      // called on every keystroke; return nil unless the query is relevant (e.g. match a keyword). Must be fast.
      MenuBar func() (title string, rows []MenuRow)          // optional; refreshed every 10s; shows in the system tray
  }
  func HTTPGet(url string) (string, error)                    // fetch a URL (5s timeout, cached 60s)

Example:
` + "```go" + `
package main

import (
	"fmt"
	"strings"
	"time"
	"nvm"
)

func Extension() nvm.Extension {
	return nvm.Extension{
		ID: "user.clock", Title: "Clock",
		Search: func(q string) []nvm.Item {
			if !strings.HasPrefix("clock", strings.ToLower(q)) || len(q) < 3 {
				return nil
			}
			now := time.Now().Format("15:04:05")
			return []nvm.Item{{Title: "Local time " + now, Subtitle: "Clock", Value: now}}
		},
	}
}
` + "```"
