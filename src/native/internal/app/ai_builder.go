package app

import (
	"context"
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// ---- extension builder: the model writes a manifest and a Go source file, the host installs and checks them ----
//
// The builder is a chat of kind builder on the smart tier. Its replies are read from the chat service; install and
// the load check stay on the extension manager. Results and retries are notes and sends on the same chat.

// builderFixAttempts bounds how many times a rejected extension goes back to the model.
const builderFixAttempts = 2

var (
	goBlockRe   = regexp.MustCompile("(?s)```(?:go|golang)?\\s*\\n(.*?)```")
	jsonBlockRe = regexp.MustCompile("(?s)```json\\s*\\n(.*?)```")
)

// extractGo returns the last Go code block of a reply.
func extractGo(s string) string {
	return lastBlock(goBlockRe, s)
}

// extractManifest returns the last JSON code block of a reply.
func extractManifest(s string) string {
	return lastBlock(jsonBlockRe, s)
}

func lastBlock(re *regexp.Regexp, s string) string {
	ms := re.FindAllStringSubmatch(s, -1)
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

// builderDone installs the extension the finished reply describes. A reply the host cannot install, or an extension
// that does not load, goes back to the model with the reason, up to builderFixAttempts retries.
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
	manifest := extractManifest(text)
	if code == "" || manifest == "" {
		return
	}
	m := p.extensionManager()
	if m == nil {
		p.noteBuilder(id, chat.NoticeFailed, "✗ Extensions are still loading; try again in a moment")
		return
	}
	extID, title, err := manifestIdentity(manifest)
	if err == nil {
		err = m.Install(extID, map[string][]byte{bundle.ManifestFile: []byte(manifest), bundle.SourceFile: []byte(code)}, true)
	}
	if err != nil {
		p.retryBuilder(id, err.Error())
		return
	}
	p.noteBuilder(id, chat.NoticeNote, "Installed “"+title+"” — checking that it runs…")
	go p.checkBuiltExtension(id, extID, title)
}

// manifestIdentity reads the id and title of the manifest the model wrote.
func manifestIdentity(manifest string) (id, title string, err error) {
	var identity struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(manifest), &identity); err != nil {
		return "", "", err
	}
	return identity.ID, identity.Title, nil
}

// checkBuiltExtension waits, off the UI thread, for the new extension's child to load or fail, then reports on
// the UI thread.
func (p *App) checkBuiltExtension(chatID, extID, title string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	m := p.extensionManager()
	var status supervisor.Status
	err := m.WaitFor(ctx, extID, func(s supervisor.Status) bool {
		status = s
		return s.State == supervisor.StateRunning || s.State == supervisor.StateBroken || s.State == supervisor.StateFailed
	})
	p.onUI(func() {
		switch {
		case err != nil:
			p.retryBuilder(chatID, "the extension did not start in time")
		case status.State == supervisor.StateRunning:
			p.noteBuilder(chatID, chat.NoticeNote, "✓ “"+title+"” is running")
		default:
			p.retryBuilder(chatID, status.Reason)
		}
	})
}

// retryBuilder sends a failure back to the model, or notes it when the retries are used up. It runs on the UI thread.
func (p *App) retryBuilder(id, reason string) {
	if p.builderTries == nil {
		p.builderTries = map[string]int{}
	}
	if p.builderTries[id] < builderFixAttempts {
		p.builderTries[id]++
		p.noteBuilder(id, chat.NoticeNote, "Extension failed ("+oneLine(reason)+") — asking the model to fix it…")
		fix := "The extension was rejected with this error:\n\n" + reason + "\n\nReply with the complete corrected manifest and file, in one ```json block and one ```go block."
		if err := p.chats.Send(id, fix); err != nil {
			log.Println("builder: could not send the fix:", err)
		}
		return
	}
	p.noteBuilder(id, chat.NoticeFailed, "✗ Could not produce a working extension: "+oneLine(reason))
}

// noteBuilder writes a builder result into the transcript as a notice.
func (p *App) noteBuilder(id string, kind chat.NoticeKind, text string) {
	if err := p.chats.Note(id, chat.Notice{Kind: kind, Title: builderNoticeTitle(kind), Message: text}); err != nil {
		log.Println("builder: could not note the result:", err)
	}
}

// builderNoticeTitle is the heading of a builder result: a failure reads as a failure, anything else as a note.
func builderNoticeTitle(kind chat.NoticeKind) string {
	if kind == chat.NoticeFailed {
		return "Extension not installed"
	}
	return "Extension"
}

// builderTemplate describes the extension API to the model. ~~~ stands for a Markdown code fence.
const builderTemplate = `You write extensions for Nevermind, a desktop command palette. An extension is a folder of two files: a manifest and one Go source file, interpreted at runtime.

Rules:
- Reply with a short sentence, then exactly two code blocks: a ~~~json manifest block, then a ~~~go source block. Nothing after them.
- Manifest: {"apiVersion":"1.0","id":"user.my-extension","title":"My extension"}. The id is lowercase and dot-separated.
- Source: under 80 lines, ` + "`package main`" + `, no goroutines, no cgo. Import "github.com/pablopunk/nvm/native/internal/ext/api" and standard-library packages only.
- Entry points (each optional; the signatures are fixed):
  func Commands(ctx api.Context) ([]api.Command, error)
  func RootItems(ctx api.Context) ([]api.Item, error)
  func SearchItems(ctx api.Context, query string) ([]api.Item, error)  // runs on every keystroke: return nil unless the query is relevant, and be fast
  func RunAction(ctx api.Context, in api.Invocation) (api.Result, error)
- api.Item{ID, Title, Subtitle, Icon string; Primary *api.Action; Actions []api.Action}. An action is api.Action{Kind, Title, Text, URL, Ref}: Kind api.ActionCopyText copies Text; api.ActionOpenURL opens URL; api.ActionRun with Ref: &api.ActionRef{ID: "x"} calls RunAction with that id.
- api.Command{ID, Title, Subtitle string}; a command runs through RunAction with its ID.

Example manifest:
~~~json
{"apiVersion":"1.0","id":"user.clock","title":"Clock"}
~~~

Example source:
~~~go
package main

import (
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if len(query) < 3 || !strings.HasPrefix("clock", strings.ToLower(query)) {
		return nil, nil
	}
	now := time.Now().Format("15:04:05")
	return []api.Item{{ID: "now", Title: "Local time " + now, Subtitle: "Clock", Primary: &api.Action{Kind: api.ActionCopyText, Text: now}}}, nil
}
~~~`

// builderSystem is the builder's system prompt, with the Markdown fence written out.
var builderSystem = strings.ReplaceAll(builderTemplate, "~~~", "```")
