package app

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// ---- extension builder: the model writes a Go extension, Yaegi checks it, we install it ----
//
// The builder is a chat of kind builder on the smart tier. Its results (installed, failed) are notes in the
// transcript. The builder moves onto the extension platform in WP3.9; until then it lives here.

var goBlockRe = regexp.MustCompile("(?s)```(?:go|golang)?\\s*\\n(.*?)```")

// builderFixAttempts bounds how many times a failing extension is sent back to the model.
const builderFixAttempts = 2

func extractGo(s string) string {
	ms := goBlockRe.FindAllStringSubmatch(s, -1)
	if len(ms) == 0 {
		return ""
	}
	return ms[len(ms)-1][1]
}

func (p *App) builderView() *palette.View { return p.builderViewWith("") }

// builderViewWith opens the extension builder and, given a prompt, sends it at once ("Automate ...").
func (p *App) builderViewWith(prompt string) *palette.View {
	return p.openChat(chat.KindBuilder, chat.TierSmart, prompt)
}

// builderDone installs the extension a finished reply contains. A reply that does not load goes back to the model
// a bounded number of times, then the failure is noted in the transcript.
func (p *App) builderDone(id string, outcome chat.Outcome) {
	if outcome.Ending != chat.EndFinished {
		return
	}
	record, ok := p.chats.Chat(id)
	if !ok {
		return
	}
	text, _ := chat.LastReply(record)
	code := extractGo(text)
	if code == "" {
		return
	}
	ext, err := evalExtension(code)
	if err != nil {
		p.retryOrNoteBuilderFailure(id, err)
		return
	}
	file := filepath.Join(extensionsDir(), sanitize(ext.ID)+".go")
	if os.WriteFile(file, []byte(code), 0o644) != nil {
		p.noteBuilder(id, chat.NoticeFailed, "✗ Could not write "+file)
		return
	}
	p.reloadExtensions()
	p.noteBuilder(id, chat.NoticeNote, "✓ Installed “"+ext.Title+"” — "+file)
}

// retryOrNoteBuilderFailure sends a non-loading extension back to the model, or notes that it gave up.
func (p *App) retryOrNoteBuilderFailure(id string, loadErr error) {
	if p.builderTries == nil {
		p.builderTries = map[string]int{}
	}
	if p.builderTries[id] < builderFixAttempts {
		p.builderTries[id]++
		p.noteBuilder(id, chat.NoticeNote, "Extension failed to load ("+oneLine(loadErr.Error())+") — asking the model to fix it…")
		fix := "The extension failed to load with this error:\n\n" + loadErr.Error() + "\n\nReply with the complete corrected file in one ```go block."
		if err := p.chats.Send(id, fix); err != nil {
			log.Println("builder: could not send the fix:", err)
		}
		return
	}
	p.noteBuilder(id, chat.NoticeFailed, "✗ Could not produce a working extension: "+oneLine(loadErr.Error()))
}

// noteBuilder writes a builder result into the transcript as a notice.
func (p *App) noteBuilder(id string, kind chat.NoticeKind, text string) {
	if err := p.chats.Note(id, chat.Notice{Kind: kind, Title: builderNoticeTitle(kind), Message: text}); err != nil {
		log.Println("builder: could not note the result:", err)
	}
}

// builderNoticeTitle is the heading of a builder result: a success reads as a note, a failure as a failure.
func builderNoticeTitle(kind chat.NoticeKind) string {
	if kind == chat.NoticeFailed {
		return "Extension not installed"
	}
	return "Extension"
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
