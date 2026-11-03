package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/search"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
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

func (p *App) newChatView(first string) *palette.View {
	ch := &Chat{ID: newID("chat"), Updated: time.Now()}
	p.chats.Chats = append([]*Chat{ch}, p.chats.Chats...)
	v := p.chatView(ch)
	if first != "" {
		p.panel.Update(func() { p.sendChat(p.runs[ch.ID], first) })
	}
	return v
}

func (p *App) chatView(ch *Chat) *palette.View {
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
	return &palette.View{
		Title: title, Placeholder: chatPlaceholder(ch), TitleFn: func() string {
			if ch.Title != "" {
				return ch.Title
			}
			return title
		},
		OnLeave:  func() { p.chats.save() },
		OnSubmit: func(q string) { p.sendChat(run, q) },
		OnKey: func(c *ui.Context, h search.Host) bool {
			if run.streaming && c.Shortcut(0, ui.KeyEscape) {
				run.cancel()
				return true
			}
			return false
		},
		Body: func(c *ui.Context, h search.Host) { p.transcript(c, run) },
	}
}

func (p *App) transcript(c *ui.Context, run *chatRun) {
	k := design.Tokens(c)
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
				ui.Text(c, hint).FontSize(13).TextColor(k.Tertiary)
			})
		} else {
			ui.List(c, &run.list, len(msgs), func(i int) {
				m := msgs[i]
				ui.Column(c).Padding(6, 4).Children(func() {
					switch m.Role {
					case "user":
						design.UserBubble(c, m.Content)
					case "notice":
						ui.Text(c, m.Content).FontSize(12).TextColor(k.Tertiary)
					default:
						if m.Content == "" {
							ui.Spinner(c)
						} else {
							replyMarkdown(c, m.Content, run.streaming && i == len(msgs)-1, &p.md)
						}
					}
				})
			}).Grow(1)
		}
	})
}

func (p *App) sendChat(run *chatRun, text string) {
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

func (p *App) chatHistoryView() *palette.View {
	return &palette.View{
		Title: "AI Chats", Placeholder: "Search AI Chats",
		Search: func(q string) []search.Item {
			lq := strings.ToLower(q)
			out := []search.Item{{Title: "New Chat", Icon: "plus", Actions: []search.Action{{Title: "Start", Run: func(h search.Host) { h.Push(p.newChatView("")) }}}}}
			for _, ch := range p.chats.Chats {
				if ch.Builder || (lq != "" && !strings.Contains(strings.ToLower(ch.Title), lq)) {
					continue
				}
				ch := ch
				out = append(out, search.Item{
					ID: ch.ID, Title: ch.Title, Subtitle: "AI conversation · " + ago(ch.Updated), Icon: "message-circle", Tone: "yellow", TintTitle: true,
					Actions: []search.Action{
						{Title: "Open", Run: func(h search.Host) { h.Push(p.chatView(ch)) }},
						{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Dismiss: search.DismissNever, Run: func(h search.Host) {
							p.chats.Chats = slices.DeleteFunc(p.chats.Chats, func(x *Chat) bool { return x.ID == ch.ID })
							p.chats.save()
							h.Refresh()
						}},
					},
				})
			}
			return out
		},
	}
}

func chatPlaceholder(ch *Chat) string {
	if ch.Builder {
		return "Describe the extension you want"
	}
	return "Message AI"
}
