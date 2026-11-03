package app

import (
	"cmp"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/search"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type engine struct{ Key, Name, URL string }

var engines = []engine{
	{"google", "Google", "https://www.google.com/search?q={query}"},
	{"duckduckgo", "DuckDuckGo", "https://duckduckgo.com/?q={query}"},
	{"bing", "Bing", "https://www.bing.com/search?q={query}"},
	{"brave", "Brave Search", "https://search.brave.com/search?q={query}"},
}

var builtinLinks = []Quicklink{
	{"GitHub", "gh", "https://github.com/search?q={query}"},
	{"YouTube", "yt", "https://www.youtube.com/results?search_query={query}"},
	{"Google Maps", "maps", "https://www.google.com/maps/search/{query}"},
	{"Wikipedia", "wiki", "https://en.wikipedia.org/w/index.php?search={query}"},
	{"npm", "npm", "https://www.npmjs.com/search?q={query}"},
	{"Go Packages", "gopkg", "https://pkg.go.dev/search?q={query}"},
}

func (p *Palette) engine() engine {
	for _, e := range engines {
		if e.Key == p.S.Engine {
			return e
		}
	}
	return engines[0]
}

func openURL(tpl, q string) func(h search.Host) {
	return func(h search.Host) {
		h.Hide()
		_ = mygo.Shell.OpenExternal(strings.ReplaceAll(tpl, "{query}", url.QueryEscape(q)))
	}
}

type command struct {
	Title, Keywords, Icon, Subtitle, Tone string
	Run                                   func(h search.Host)
	Order                                 int
	Hidden                                bool
}

func (p *Palette) commands() []command {
	home, _ := os.UserHomeDir()
	place := func(name string) command {
		dir := filepath.Join(home, name)
		return command{Title: name, Keywords: "open folder places go", Icon: "folder", Subtitle: dir, Hidden: true, Run: func(h search.Host) { h.Hide(); _ = mygo.Shell.OpenPath(dir) }}
	}
	cmds := []command{
		{Title: "Clipboard History", Keywords: "clipboard paste copy history", Icon: "clipboard", Tone: "blue", Subtitle: "Browse and paste what you copied", Run: func(h search.Host) { h.Push(p.clipboardView()) }},
		{Title: "AI Chats", Keywords: "ai chat assistant gpt llm ask conversations history", Icon: "sparkles", Tone: "yellow", Subtitle: "Chat with the assistant", Run: func(h search.Host) { h.Push(p.chatHistoryView()) }},
		{Title: "Emoji & Symbols", Keywords: "emoji symbols unicode smiley", Icon: "smile", Subtitle: "Search and paste emoji", Run: func(h search.Host) { h.Push(p.emojiView()) }},
		{Title: "Search Files", Keywords: "files find documents locate", Icon: "folder", Subtitle: "Find files in your home folder", Run: func(h search.Host) { h.Push(p.filesView()) }},
		{Title: "Quick Notes", Keywords: "notes note scratch memo floating", Icon: "sticky-note", Subtitle: "Jot something down", Run: func(h search.Host) { h.Push(p.notesView()) }},
		{Title: "New Note", Keywords: "note create scratch", Icon: "plus", Hidden: true, Run: func(h search.Host) { p.newNote() }},
		{Title: "Snippets", Keywords: "snippets text expand template", Icon: "type", Subtitle: "Saved text you paste often", Run: func(h search.Host) { h.Push(p.snippetsView()) }},
		{Title: "Create Snippet", Keywords: "snippet new add", Icon: "plus", Hidden: true, Run: func(h search.Host) { h.Push(p.snippetForm(nil)) }},
		{Title: "Extensions", Keywords: "extensions manage plugins create builder", Icon: "puzzle", Subtitle: "Manage and create extensions", Run: func(h search.Host) { h.Push(p.extensionsView()) }},
		{Title: "Quit Applications", Keywords: "quit kill processes force apps", Icon: "power", Subtitle: "Stop running processes", Run: func(h search.Host) { h.Push(p.processesView()) }},
		{Title: "Create Quicklink", Keywords: "quicklink bookmark url search new", Icon: "link", Hidden: true, Run: func(h search.Host) { h.Push(p.quicklinkForm()) }},
		{Title: "Start Timer", Keywords: "timer countdown alarm", Icon: "timer", Subtitle: "Type e.g. “timer 5m”", Hidden: true, Run: func(h search.Host) { p.query = "timer 5m"; p.search() }},
		{Title: "Lock Screen", Keywords: "lock screen security", Icon: "lock", Hidden: true, Run: func(h search.Host) { h.Hide(); run("loginctl", "lock-session") }},
		{Title: "Sleep", Keywords: "sleep suspend", Icon: "moon", Hidden: true, Run: func(h search.Host) { h.Hide(); run("systemctl", "suspend") }},
		{Title: "Log Out", Keywords: "logout sign out", Icon: "log-out", Hidden: true, Run: func(h search.Host) {
			h.Push(p.confirmView("Log Out", "End the current session", func() { run("loginctl", "terminate-user", os.Getenv("USER")) }))
		}},
		{Title: "Restart", Keywords: "reboot restart", Icon: "rotate-ccw", Hidden: true, Run: func(h search.Host) {
			h.Push(p.confirmView("Restart", "Reboot this computer", func() { run("systemctl", "reboot") }))
		}},
		{Title: "Shut Down", Keywords: "shutdown power off", Icon: "power", Hidden: true, Run: func(h search.Host) {
			h.Push(p.confirmView("Shut Down", "Power off this computer", func() { run("systemctl", "poweroff") }))
		}},
		{Title: "Empty Trash", Keywords: "trash delete bin empty", Icon: "trash-2", Tone: "red", Hidden: true, Run: func(h search.Host) {
			h.Push(p.confirmView("Empty Trash", "Permanently delete trashed files", func() { p.emptyTrash() }))
		}},
		{Title: "Settings", Keywords: "settings preferences config hotkey", Icon: "settings", Subtitle: "Hotkey, search engine, clipboard", Run: func(h search.Host) { h.Push(p.settingsView()) }},
		{Title: "Quit Nevermind", Keywords: "quit exit nevermind", Icon: "x", Hidden: true, Run: func(h search.Host) { mygo.App.Quit() }},
		place("Downloads"), place("Documents"), place("Desktop"), place("Pictures"),
	}
	if p.timer != nil {
		cmds = append(cmds, command{Title: "Cancel Timer", Keywords: "timer cancel stop", Icon: "timer", Hidden: true, Run: func(h search.Host) { p.cancelTimer(); h.Hide() }})
	}
	tones := map[string]string{"Clipboard History": "blue", "AI Chats": "purple", "Emoji & Symbols": "yellow", "Search Files": "blue", "Quick Notes": "yellow",
		"New Note": "yellow", "Snippets": "teal", "Create Snippet": "teal", "Extensions": "purple", "Quit Applications": "red", "Create Quicklink": "blue",
		"Start Timer": "orange", "Sleep": "indigo", "Restart": "orange", "Shut Down": "red", "Empty Trash": "red", "Downloads": "blue", "Documents": "blue",
		"Desktop": "blue", "Pictures": "blue", "Cancel Timer": "orange"}
	for i := range cmds {
		cmds[i].Order = i
		cmds[i].Tone = tones[cmds[i].Title]
	}
	return cmds
}

type scored struct {
	it    search.Item
	score int
}

func (p *Palette) rootSearch(q string) []search.Item {
	lq := strings.ToLower(q)
	var out []scored
	push := func(s int, it search.Item) { out = append(out, scored{it, s}) }

	calcShown := false
	if disp, cp, ok := calcResult(q); ok {
		calcShown = true
		push(1000, search.Item{Kind: "Calculator", Title: disp, Subtitle: "Copy result", Icon: "calculator", Tone: "orange", Actions: []search.Action{
			{Title: "Copy Result", Run: func(h search.Host) { mygo.Clipboard.WriteText(cp); h.Hide() }},
			{Title: "Paste Result", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) { h.Hide(); pasteText(cp) }},
		}})
	}
	if d, ok := parseTimer(q); ok && d > 0 {
		push(990, search.Item{Kind: "Timer", Title: "Start timer: " + d.String(), Subtitle: "Shows a countdown in the tray", Icon: "timer", Tone: "orange", Actions: []search.Action{{Title: "Start", Run: func(h search.Host) { p.startTimer(d); h.Hide() }}}})
	}
	if kw, rest, ok := strings.Cut(q, " "); ok && strings.TrimSpace(rest) != "" {
		for _, ql := range append(slices.Clone(p.S.Quicklinks), builtinLinks...) {
			if strings.EqualFold(kw, ql.Keyword) {
				push(980, search.Item{Kind: "Quicklinks", Title: fmt.Sprintf("Search %s for “%s”", ql.Name, strings.TrimSpace(rest)), Subtitle: "Quicklink", Icon: "link", Tone: "blue",
					Actions: []search.Action{{Title: "Open", Run: openURL(ql.URL, strings.TrimSpace(rest))}}})
			}
		}
	}
	for _, sn := range p.snips.Items {
		if sn.Keyword != "" && lq != "" && strings.HasPrefix(strings.ToLower(sn.Keyword), lq) {
			it := p.snippetItem(sn)
			it.Kind = "Snippets"
			push(970, it)
		}
	}
	for _, e := range p.exts {
		if p.S.Disabled[e.ID] {
			continue
		}
		for _, x := range safeSearch(e.Extension, q) {
			x := x
			act := search.Action{Title: "Copy", Run: func(h search.Host) { mygo.Clipboard.WriteText(x.Value); h.Hide() }}
			if x.URL != "" {
				act = search.Action{Title: "Open", Run: func(h search.Host) { h.Hide(); _ = mygo.Shell.OpenExternal(x.URL) }}
			}
			push(960, search.Item{Kind: e.Title, Title: x.Title, Subtitle: x.Subtitle, Icon: "puzzle", Tone: "purple", Actions: []search.Action{act}})
		}
	}
	// commands: ranked by title (and, below, by keywords)
	for _, cmd := range p.commands() {
		cmd := cmd
		it := search.Item{Kind: "Commands", Title: cmd.Title, Subtitle: cmd.Subtitle, Icon: cmd.Icon, Tone: cmd.Tone, Actions: []search.Action{{Title: "Open", Run: cmd.Run}}}
		if lq == "" {
			if !cmd.Hidden {
				push(100-cmd.Order, it)
			}
			continue
		}
		s := search.Score(cmd.Title, lq)
		if s == 0 && len(lq) >= 2 && strings.Contains(strings.ToLower(cmd.Keywords), lq) {
			s = 45
		}
		if s > 0 {
			push(s+boost(p, "cmd:"+cmd.Title), it)
		}
	}
	for _, a := range p.apps {
		a := a
		s := 0
		if lq == "" {
			if p.S.Usage[a.ID] == 0 {
				continue
			}
			s = 60
		} else if s = search.Score(a.Name, lq); s == 0 {
			continue
		}
		push(s+min(p.S.Usage[a.ID], 9), search.Item{Kind: "Applications", Title: a.Name, IconPath: a.Path, Actions: []search.Action{
			{Title: "Open Application", Run: func(h search.Host) {
				h.Hide()
				p.S.Usage[a.ID]++
				p.S.save()
				if err := mygo.Shell.LaunchApp(a); err != nil {
					h.Toast(err.Error())
				}
			}},
			{Title: "Show in Folder", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) { h.Hide(); mygo.Shell.ShowItemInFolder(a.Path) }},
		}})
	}
	if len(lq) >= 2 && p.files.done {
		for i, path := range p.files.search(q, 4) {
			fi := p.fileItem(path)
			fi.Kind = "Files"
			push(75-i, fi)
		}
	}
	if len(lq) >= 3 {
		n := 0
		for _, e := range p.clip.Entries {
			if e.Kind == "text" && strings.Contains(strings.ToLower(e.Text), lq) && n < 2 {
				ci := p.clipItem(e)
				ci.Kind = "Clipboard"
				push(55-n, ci)
				n++
			}
		}
	}
	// recent AI conversations
	nchat := 0
	for _, ch := range p.chats.Chats {
		if ch.Builder || ch.Title == "" || nchat >= 4 {
			continue
		}
		ch := ch
		s := 30
		if lq != "" {
			if s = search.Score(ch.Title, lq); s == 0 {
				continue
			}
		}
		nchat++
		push(s, search.Item{Kind: "Conversations", Title: "Continue conversation: " + oneLine(ch.Title), Subtitle: "AI conversation", Icon: "message-circle", Tone: "yellow", TintTitle: true,
			Actions: []search.Action{{Title: "Open", Run: func(h search.Host) { h.Push(p.chatView(ch)) }}}})
	}
	// URL, then the query fallbacks (web search + Automate)
	if q != "" {
		if u := search.URLFromQuery(q); u != "" {
			push(100+50, search.Item{Kind: "Web", Title: "Open " + strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://"), Subtitle: "Open website", Icon: "globe", Tone: "blue",
				Actions: []search.Action{{Title: "Open Website", Run: func(h search.Host) { h.Hide(); _ = mygo.Shell.OpenExternal(u) }}}})
		} else if !calcShown {
			eng := p.engine()
			push(40, search.Item{Kind: "Search", Title: `Automate "` + oneLine(q) + `"`, Subtitle: "Build a reusable command with AI", Icon: "zap", Tone: "yellow", Prominent: true,
				Actions: []search.Action{{Title: "Automate", Run: func(h search.Host) { h.Push(p.builderViewWith(q)) }}}})
			push(10+boost(p, "web:"+q), search.Item{Kind: "Search", Title: `Search the web for "` + oneLine(q) + `"`, Subtitle: "Search instead", Icon: "search", Tone: "blue",
				Actions: []search.Action{{Title: "Search the Web", Run: openURL(eng.URL, q)}}})
		}
	}
	slices.SortStableFunc(out, func(a, b scored) int { return cmp.Compare(b.score, a.score) })
	items := make([]search.Item, 0, len(out))
	for _, s := range out {
		items = append(items, s.it)
		if len(items) >= 40 {
			break
		}
	}
	return sectioned(items, q != "")
}

func boost(p *Palette, key string) int { return min(p.S.Usage[key], 8) }

func (p *Palette) rootView() *View {
	return &View{Title: "Nevermind", Search: p.rootSearch, Hint: "esc Close"}
}

// ---- settings ----

func (p *Palette) settingsView() *View {
	return &View{
		Title: "Settings", Placeholder: "Search Settings",
		Search: func(q string) []search.Item {
			lq := strings.ToLower(q)
			auth := "Not signed in"
			if a := loadAuth(); a != nil {
				auth = "Signed in" + func() string {
					if a.Email != "" {
						return " as " + a.Email
					}
					return ""
				}()
			}
			clip := "On"
			if p.S.ClipOff {
				clip = "Off"
			}
			login := "Off"
			if autostartEnabled() {
				login = "On"
			}
			rows := []search.Item{
				{Title: "Palette Hotkey", Accessory: p.S.Hotkey, Icon: "keyboard", Actions: []search.Action{{Title: "Change…", Run: func(h search.Host) { h.Push(p.hotkeyView()) }}}},
				{Title: "Default Search Engine", Accessory: p.engine().Name, Icon: "search", Actions: []search.Action{{Title: "Next Engine", Run: func(h search.Host) {
					i := slices.IndexFunc(engines, func(e engine) bool { return e.Key == p.S.Engine })
					p.S.Engine = engines[(i+1)%len(engines)].Key
					p.S.save()
					h.Refresh()
				}}}},
				{Title: "Clipboard History", Accessory: clip, Icon: "clipboard", Actions: []search.Action{{Title: "Toggle", Run: func(h search.Host) { p.S.ClipOff = !p.S.ClipOff; p.S.save(); h.Refresh() }}}},
				{Title: "Clipboard History Size", Accessory: itoa(p.S.ClipLimit), Icon: "clipboard", Actions: []search.Action{{Title: "Cycle", Run: func(h search.Host) {
					sizes := []int{50, 200, 500, 1000}
					i := slices.Index(sizes, p.S.ClipLimit)
					p.S.ClipLimit = sizes[(i+1)%len(sizes)]
					p.S.save()
					h.Refresh()
				}}}},
				{Title: "Launch at Login", Accessory: login, Icon: "power", Actions: []search.Action{{Title: "Toggle", Run: func(h search.Host) { setAutostart(!autostartEnabled()); h.Refresh() }}}},
				{Title: "Account", Accessory: auth, Icon: "user", Actions: []search.Action{{Title: "Show Auth File", Run: func(h search.Host) { h.Hide(); mygo.Shell.ShowItemInFolder(filepath.Join(configDir(), "auth.json")) }}}},
				{Title: "Open Config Folder", Subtitle: configDir(), Icon: "folder", Actions: []search.Action{{Title: "Open", Run: func(h search.Host) { h.Hide(); _ = mygo.Shell.OpenPath(configDir()) }}}},
			}
			if lq == "" {
				return rows
			}
			return slices.DeleteFunc(rows, func(it search.Item) bool { return !strings.Contains(strings.ToLower(it.Title), lq) })
		},
	}
}

func (p *Palette) hotkeyView() *View {
	return &View{
		Title: "Hotkey", Placeholder: "Press keys, e.g. Alt+Shift+Space",
		Search: func(q string) []search.Item {
			if q == "" {
				return []search.Item{{Title: "Type a new accelerator", Subtitle: "Current: " + p.S.Hotkey, Icon: "keyboard"}}
			}
			return []search.Item{{Title: "Use “" + q + "”", Icon: "keyboard", Actions: []search.Action{{Title: "Save", Run: func(h search.Host) {
				old := p.S.Hotkey
				mygo.GlobalShortcut.Unregister(old)
				if err := mygo.GlobalShortcut.Register(q, p.toggle); err != nil {
					_ = mygo.GlobalShortcut.Register(old, p.toggle)
					h.Toast("Invalid or taken: " + err.Error())
					return
				}
				p.S.Hotkey = q
				p.S.save()
				h.Pop()
				h.Toast("Hotkey set to " + q)
			}}}}}
		},
	}
}

func autostartPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autostart", "nvm-native.desktop")
}

func autostartEnabled() bool { _, err := os.Stat(autostartPath()); return err == nil }

func setAutostart(on bool) {
	if !on {
		_ = os.Remove(autostartPath())
		return
	}
	exe, _ := os.Executable()
	_ = os.MkdirAll(filepath.Dir(autostartPath()), 0o755)
	_ = os.WriteFile(autostartPath(), []byte("[Desktop Entry]\nType=Application\nName=Nevermind\nExec="+exe+"\nX-GNOME-Autostart-enabled=true\n"), 0o644)
}

var sectionOrder = []string{"Calculator", "Timer", "Quicklinks", "Snippets", "Commands", "Applications", "Files", "Clipboard", "Conversations", "Web", "Search"}

func sectionRank(kind string) int {
	if i := slices.Index(sectionOrder, kind); i >= 0 {
		return i
	}
	return 4 // extensions sit with commands
}

// sectioned labels runs of results like Spotlight: a "Top Hit", then groups by kind.
func sectioned(items []search.Item, searching bool) []search.Item {
	if len(items) == 0 {
		return items
	}
	var top []search.Item
	rest := items
	if searching {
		top, rest = items[:1], slices.Clone(items[1:])
	} else {
		rest = slices.Clone(items)
	}
	slices.SortStableFunc(rest, func(a, b search.Item) int { return cmp.Compare(sectionRank(a.Kind), sectionRank(b.Kind)) })
	out := make([]search.Item, 0, len(items))
	for _, it := range top {
		it.Section = "Top Hit"
		out = append(out, it)
	}
	for _, it := range rest {
		it.Section = it.Kind
		if !searching && it.Kind == "Commands" {
			it.Section = "Suggestions"
		}
		out = append(out, it)
	}
	return out
}
