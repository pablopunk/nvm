package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

type ChatMsg struct{ Role, Content string } // user | assistant | notice

type Chat struct {
	ID       string
	Title    string
	Messages []ChatMsg
	Updated  time.Time
	Builder  bool
}

type ChatStore struct{ Chats []*Chat }

func newChatStore() *ChatStore {
	s := &ChatStore{}
	loadJSON("chats.json", s)
	return s
}

func (s *ChatStore) save() { saveJSON("chats.json", s) }

type authInfo struct{ BaseURL, Token, Email string }

func loadAuth() *authInfo {
	a := &authInfo{}
	loadJSON("auth.json", a)
	if a.Token == "" {
		if t := os.Getenv("NVM_TOKEN"); t != "" {
			a.Token, a.BaseURL = t, "https://api.nvm.fyi"
		}
	}
	if a.Token == "" || a.BaseURL == "" {
		return nil
	}
	return a
}

type chatRun struct {
	chat      *Chat
	streaming bool
	cancel    context.CancelFunc
	list      ui.ListState
	attempts  int
}

const baseSystem = "You are Nevermind, an assistant inside a desktop command palette. Be concise and direct; use Markdown."

func (p *Palette) newChatView(first string) *View {
	ch := &Chat{ID: newID("chat"), Updated: time.Now()}
	p.chats.Chats = append([]*Chat{ch}, p.chats.Chats...)
	v := p.chatView(ch)
	if first != "" {
		p.panel.Update(func() { p.sendChat(p.runs[ch.ID], first) })
	}
	return v
}

func (p *Palette) chatView(ch *Chat) *View {
	run := &chatRun{chat: ch}
	run.list.FollowEnd = true
	if p.runs == nil {
		p.runs = map[string]*chatRun{}
	}
	p.runs[ch.ID] = run
	title := ch.Title
	if title == "" {
		title = "AI Chat"
		if ch.Builder {
			title = "Create Extension"
		}
	}
	return &View{
		Title: title, Placeholder: chatPlaceholder(ch), TitleFn: func() string {
			if ch.Title != "" {
				return ch.Title
			}
			return title
		},
		OnLeave:  func() { p.chats.save() },
		OnSubmit: func(q string) { p.sendChat(run, q) },
		OnKey: func(c *ui.Context, p *Palette) bool {
			if run.streaming && c.Shortcut(0, ui.KeyEscape) {
				run.cancel()
				return true
			}
			return false
		},
		Body: func(c *ui.Context, p *Palette) { p.transcript(c, run) },
	}
}

func (p *Palette) transcript(c *ui.Context, run *chatRun) {
	k := tk(c)
	msgs := run.chat.Messages
	ui.Column(c).Grow(1).Padding(8, 16, 8, 16).Children(func() {
		if len(msgs) == 0 {
			ui.Column(c).Grow(1).Center().Gap(4).Children(func() {
				hint := "Ask anything. Replies stream in as Markdown."
				if run.chat.Builder {
					hint = "Describe what you want; the model writes a Go extension, it is checked and installed."
				}
				if loadAuth() == nil {
					hint = "Not signed in: put a token in ~/.config/nvm-native/auth.json"
				}
				ui.Text(c, hint).FontSize(13).TextColor(k.tertiary)
			})
		} else {
			ui.List(c, &run.list, len(msgs), func(i int) {
				m := msgs[i]
				ui.Column(c).Padding(6, 4).Children(func() {
					switch m.Role {
					case "user":
						ui.Row(c).Children(func() {
							ui.Spacer(c)
							ui.Text(c, m.Content).FontSize(13).TextColor(k.strong).Padding(8, 12).Radius(14).Background(k.interactive).Border(1, k.edgeSubtle).InsetShadow(0, 1, 0, 0, k.hi).MaxWidth(520)
						})
					case "notice":
						ui.Text(c, m.Content).FontSize(12).TextColor(k.tertiary)
					default:
						if m.Content == "" {
							ui.Spinner(c)
						} else if run.streaming && i == len(msgs)-1 {
							markdown.Stream(c, m.Content, &p.md)
						} else {
							markdown.View(c, m.Content, &p.md)
						}
					}
				})
			}).Grow(1)
		}
	})
}

func (p *Palette) sendChat(run *chatRun, text string) {
	if run.streaming || strings.TrimSpace(text) == "" {
		return
	}
	ch := run.chat
	if ch.Title == "" {
		ch.Title = oneLine(text)
	}
	ch.Messages = append(ch.Messages, ChatMsg{"user", text}, ChatMsg{"assistant", ""})
	ch.Updated = time.Now()
	run.streaming = true
	system := baseSystem
	if ch.Builder {
		system = builderSystem
	}
	var api []map[string]string
	api = append(api, map[string]string{"role": "system", "content": system})
	for _, m := range ch.Messages[:len(ch.Messages)-1] {
		if m.Role != "notice" {
			api = append(api, map[string]string{"role": m.Role, "content": m.Content})
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	run.cancel = cancel
	go func() {
		finish, err := streamCompletion(ctx, api, func(chunk string) {
			p.panel.Update(func() { ch.Messages[len(ch.Messages)-1].Content += chunk })
		})
		p.panel.Update(func() {
			run.streaming = false
			last := &ch.Messages[len(ch.Messages)-1]
			if err != nil && ctx.Err() == nil {
				last.Content += "\n\n⚠ " + err.Error()
			}
			if last.Content == "" {
				ch.Messages = ch.Messages[:len(ch.Messages)-1]
			}
			p.chats.save()
			if ch.Builder && err == nil {
				if finish == "length" && extractGo(last.Content) == "" && run.attempts < 2 {
					run.attempts++
					p.notice(run, "Reply was cut off — asking for a shorter version…")
					p.sendChat(run, "Your reply was cut off. Reply again with the complete extension in one ```go block, as short as possible, no commentary.")
					return
				}
				p.builderDone(run)
			}
		})
	}()
}

func streamCompletion(ctx context.Context, msgs []map[string]string, onChunk func(string)) (finish string, err error) {
	a := loadAuth()
	if a == nil {
		return "", fmt.Errorf("not signed in — add a token to ~/.config/nvm-native/auth.json")
	}
	body, _ := json.Marshal(map[string]any{"model": "auto", "stream": true, "max_tokens": 4096, "messages": msgs})
	req, _ := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(a.BaseURL, "/")+"/api/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Nevermind-Client", "desktop")
	req.Header.Set("X-Nevermind-Client-Version", "2.0.0-go")
	req.Header.Set("X-Nevermind-API-Version", "1")
	req.Header.Set("X-Nevermind-Platform", "linux")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var e struct{ Error struct{ Message string } }
		if json.Unmarshal(b, &e) == nil && e.Error.Message != "" {
			return "", fmt.Errorf("%s", e.Error.Message)
		}
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	got := false
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		line := sc.Text()
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "[DONE]" {
			continue
		}
		var ev struct {
			Choices []struct {
				Delta        struct{ Content string }
				FinishReason string `json:"finish_reason"`
			}
			Error struct{ Message string }
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		if ev.Error.Message != "" {
			return finish, fmt.Errorf("%s", ev.Error.Message)
		}
		if len(ev.Choices) > 0 {
			if ev.Choices[0].Delta.Content != "" {
				got = true
				onChunk(ev.Choices[0].Delta.Content)
			}
			if r := ev.Choices[0].FinishReason; r != "" {
				finish = r
			}
		}
	}
	if err := sc.Err(); err != nil {
		return finish, err
	}
	if !got {
		log.Printf("empty completion, finish_reason=%q", finish)
		return finish, fmt.Errorf("the model returned no answer (finish: %s) — try again", finish)
	}
	return finish, nil
}

func (p *Palette) chatHistoryView() *View {
	return &View{
		Title: "AI Chats", Placeholder: "Search AI Chats",
		Search: func(q string) []Item {
			lq := strings.ToLower(q)
			out := []Item{{Title: "New Chat", Icon: "plus", Actions: []Action{{Title: "Start", Run: func(p *Palette) { p.Push(p.newChatView("")) }}}}}
			for _, ch := range p.chats.Chats {
				if ch.Builder || (lq != "" && !strings.Contains(strings.ToLower(ch.Title), lq)) {
					continue
				}
				ch := ch
				out = append(out, Item{
					ID: ch.ID, Title: ch.Title, Subtitle: "AI conversation · " + ago(ch.Updated), Icon: "message-circle", Tone: "yellow", TintTitle: true,
					Actions: []Action{
						{Title: "Open", Run: func(p *Palette) { p.Push(p.chatView(ch)) }},
						{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Run: func(p *Palette) {
							p.chats.Chats = slices.DeleteFunc(p.chats.Chats, func(x *Chat) bool { return x.ID == ch.ID })
							p.chats.save()
							p.Refresh()
						}},
					},
				})
			}
			return out
		},
	}
}

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

func chatPlaceholder(ch *Chat) string {
	if ch.Builder {
		return "Describe the extension you want"
	}
	return "Message AI"
}
