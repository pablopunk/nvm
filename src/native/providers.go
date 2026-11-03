package main

import (
	"cmp"
	"fmt"
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

func openURL(tpl, q string) func(p *Palette) {
	return func(p *Palette) {
		p.Hide()
		_ = mygo.Shell.OpenExternal(strings.ReplaceAll(tpl, "{query}", url.QueryEscape(q)))
	}
}

type command struct {
	Title, Keywords, Icon, Subtitle, Tone string
	Run                                   func(p *Palette)
	Order                                 int
	Hidden                                bool
}

func (p *Palette) commands() []command {
	home, _ := os.UserHomeDir()
	place := func(name string) command {
		dir := filepath.Join(home, name)
		return command{Title: name, Keywords: "open folder places go", Icon: "folder", Subtitle: dir, Hidden: true, Run: func(p *Palette) { p.Hide(); _ = mygo.Shell.OpenPath(dir) }}
	}
	cmds := []command{
		{Title: "Clipboard History", Keywords: "clipboard paste copy history", Icon: "clipboard", Tone: "blue", Subtitle: "Browse and paste what you copied", Run: func(p *Palette) { p.Push(p.clipboardView()) }},
		{Title: "AI Chats", Keywords: "ai chat assistant gpt llm ask conversations history", Icon: "sparkles", Tone: "yellow", Subtitle: "Chat with the assistant", Run: func(p *Palette) { p.Push(p.chatHistoryView()) }},
		{Title: "Emoji & Symbols", Keywords: "emoji symbols unicode smiley", Icon: "smile", Subtitle: "Search and paste emoji", Run: func(p *Palette) { p.Push(p.emojiView()) }},
		{Title: "Search Files", Keywords: "files find documents locate", Icon: "folder", Subtitle: "Find files in your home folder", Run: func(p *Palette) { p.Push(p.filesView()) }},
		{Title: "Quick Notes", Keywords: "notes note scratch memo floating", Icon: "sticky-note", Subtitle: "Jot something down", Run: func(p *Palette) { p.Push(p.notesView()) }},
		{Title: "New Note", Keywords: "note create scratch", Icon: "plus", Hidden: true, Run: func(p *Palette) { p.newNote() }},
		{Title: "Snippets", Keywords: "snippets text expand template", Icon: "type", Subtitle: "Saved text you paste often", Run: func(p *Palette) { p.Push(p.snippetsView()) }},
		{Title: "Create Snippet", Keywords: "snippet new add", Icon: "plus", Hidden: true, Run: func(p *Palette) { p.Push(p.snippetForm(nil)) }},
		{Title: "Extensions", Keywords: "extensions manage plugins create builder", Icon: "puzzle", Subtitle: "Manage and create extensions", Run: func(p *Palette) { p.Push(p.extensionsView()) }},
		{Title: "Quit Applications", Keywords: "quit kill processes force apps", Icon: "power", Subtitle: "Stop running processes", Run: func(p *Palette) { p.Push(p.processesView()) }},
		{Title: "Create Quicklink", Keywords: "quicklink bookmark url search new", Icon: "link", Hidden: true, Run: func(p *Palette) { p.Push(p.quicklinkForm()) }},
		{Title: "Start Timer", Keywords: "timer countdown alarm", Icon: "timer", Subtitle: "Type e.g. “timer 5m”", Hidden: true, Run: func(p *Palette) { p.query = "timer 5m"; p.search() }},
		{Title: "Lock Screen", Keywords: "lock screen security", Icon: "lock", Hidden: true, Run: func(p *Palette) { p.Hide(); run("loginctl", "lock-session") }},
		{Title: "Sleep", Keywords: "sleep suspend", Icon: "moon", Hidden: true, Run: func(p *Palette) { p.Hide(); run("systemctl", "suspend") }},
		{Title: "Log Out", Keywords: "logout sign out", Icon: "log-out", Hidden: true, Run: func(p *Palette) {
			p.Push(p.confirmView("Log Out", "End the current session", func() { run("loginctl", "terminate-user", os.Getenv("USER")) }))
		}},
		{Title: "Restart", Keywords: "reboot restart", Icon: "rotate-ccw", Hidden: true, Run: func(p *Palette) {
			p.Push(p.confirmView("Restart", "Reboot this computer", func() { run("systemctl", "reboot") }))
		}},
		{Title: "Shut Down", Keywords: "shutdown power off", Icon: "power", Hidden: true, Run: func(p *Palette) {
			p.Push(p.confirmView("Shut Down", "Power off this computer", func() { run("systemctl", "poweroff") }))
		}},
		{Title: "Empty Trash", Keywords: "trash delete bin empty", Icon: "trash-2", Tone: "red", Hidden: true, Run: func(p *Palette) {
			p.Push(p.confirmView("Empty Trash", "Permanently delete trashed files", func() { p.emptyTrash() }))
		}},
		{Title: "Settings", Keywords: "settings preferences config hotkey", Icon: "settings", Subtitle: "Hotkey, search engine, clipboard", Run: func(p *Palette) { p.Push(p.settingsView()) }},
		{Title: "Quit Nevermind", Keywords: "quit exit nevermind", Icon: "x", Hidden: true, Run: func(p *Palette) { mygo.App.Quit() }},
		place("Downloads"), place("Documents"), place("Desktop"), place("Pictures"),
	}
	if p.timer != nil {
		cmds = append(cmds, command{Title: "Cancel Timer", Keywords: "timer cancel stop", Icon: "timer", Hidden: true, Run: func(p *Palette) { p.cancelTimer(); p.Hide() }})
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

func rank(name, q string) int {
	switch {
	case q == "" || strings.HasPrefix(name, q):
		return 0
	case strings.Contains(name, " "+q) || strings.Contains(name, "-"+q):
		return 1
	case strings.Contains(name, q):
		return 2
	}
	rest := name
	for _, r := range q {
		i := strings.IndexRune(rest, r)
		if i < 0 {
			return -1
		}
		rest = rest[i+len(string(r)):]
	}
	return 3
}

type scored struct {
	it    Item
	score int
}

func (p *Palette) rootSearch(q string) []Item {
	lq := strings.ToLower(q)
	var out []scored
	push := func(s int, it Item) { out = append(out, scored{it, s}) }

	calcShown := false
	if disp, cp, ok := calcResult(q); ok {
		calcShown = true
		push(1000, Item{Kind: "Calculator", Title: disp, Subtitle: "Copy result", Icon: "calculator", Tone: "orange", Actions: []Action{
			{Title: "Copy Result", Run: func(p *Palette) { mygo.Clipboard.WriteText(cp); p.Hide() }},
			{Title: "Paste Result", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(p *Palette) { p.Hide(); pasteText(cp) }},
		}})
	}
	if d, ok := parseTimer(q); ok && d > 0 {
		push(990, Item{Kind: "Timer", Title: "Start timer: " + d.String(), Subtitle: "Shows a countdown in the tray", Icon: "timer", Tone: "orange", Actions: []Action{{Title: "Start", Run: func(p *Palette) { p.startTimer(d); p.Hide() }}}})
	}
	if kw, rest, ok := strings.Cut(q, " "); ok && strings.TrimSpace(rest) != "" {
		for _, ql := range append(slices.Clone(p.S.Quicklinks), builtinLinks...) {
			if strings.EqualFold(kw, ql.Keyword) {
				push(980, Item{Kind: "Quicklinks", Title: fmt.Sprintf("Search %s for “%s”", ql.Name, strings.TrimSpace(rest)), Subtitle: "Quicklink", Icon: "link", Tone: "blue",
					Actions: []Action{{Title: "Open", Run: openURL(ql.URL, strings.TrimSpace(rest))}}})
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
			act := Action{Title: "Copy", Run: func(p *Palette) { mygo.Clipboard.WriteText(x.Value); p.Hide() }}
			if x.URL != "" {
				act = Action{Title: "Open", Run: func(p *Palette) { p.Hide(); _ = mygo.Shell.OpenExternal(x.URL) }}
			}
			push(960, Item{Kind: e.Title, Title: x.Title, Subtitle: x.Subtitle, Icon: "puzzle", Tone: "purple", Actions: []Action{act}})
		}
	}
	// commands: ranked by title (and, below, by keywords)
	for _, cmd := range p.commands() {
		cmd := cmd
		it := Item{Kind: "Commands", Title: cmd.Title, Subtitle: cmd.Subtitle, Icon: cmd.Icon, Tone: cmd.Tone, Actions: []Action{{Title: "Open", Run: cmd.Run}}}
		if lq == "" {
			if !cmd.Hidden {
				push(100-cmd.Order, it)
			}
			continue
		}
		s := score(cmd.Title, lq)
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
		} else if s = score(a.Name, lq); s == 0 {
			continue
		}
		push(s+min(p.S.Usage[a.ID], 9), Item{Kind: "Applications", Title: a.Name, IconPath: a.Path, Actions: []Action{
			{Title: "Open Application", Run: func(p *Palette) {
				p.Hide()
				p.S.Usage[a.ID]++
				p.S.save()
				if err := mygo.Shell.LaunchApp(a); err != nil {
					p.Toast(err.Error())
				}
			}},
			{Title: "Show in Folder", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(p *Palette) { p.Hide(); mygo.Shell.ShowItemInFolder(a.Path) }},
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
			if s = score(ch.Title, lq); s == 0 {
				continue
			}
		}
		nchat++
		push(s, Item{Kind: "Conversations", Title: "Continue conversation: " + oneLine(ch.Title), Subtitle: "AI conversation", Icon: "message-circle", Tone: "yellow", TintTitle: true,
			Actions: []Action{{Title: "Open", Run: func(p *Palette) { p.Push(p.chatView(ch)) }}}})
	}
	// URL, then the query fallbacks (web search + Automate)
	if q != "" {
		if u := urlFromQuery(q); u != "" {
			push(100+50, Item{Kind: "Web", Title: "Open " + strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://"), Subtitle: "Open website", Icon: "globe", Tone: "blue",
				Actions: []Action{{Title: "Open Website", Run: func(p *Palette) { p.Hide(); _ = mygo.Shell.OpenExternal(u) }}}})
		} else if !calcShown {
			eng := p.engine()
			push(40, Item{Kind: "Search", Title: `Automate "` + oneLine(q) + `"`, Subtitle: "Build a reusable command with AI", Icon: "zap", Tone: "yellow", Prominent: true,
				Actions: []Action{{Title: "Automate", Run: func(p *Palette) { p.Push(p.builderViewWith(q)) }}}})
			push(10+boost(p, "web:"+q), Item{Kind: "Search", Title: `Search the web for "` + oneLine(q) + `"`, Subtitle: "Search instead", Icon: "search", Tone: "blue",
				Actions: []Action{{Title: "Search the Web", Run: openURL(eng.URL, q)}}})
		}
	}
	slices.SortStableFunc(out, func(a, b scored) int { return cmp.Compare(b.score, a.score) })
	items := make([]Item, 0, len(out))
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
		Search: func(q string) []Item {
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
			rows := []Item{
				{Title: "Palette Hotkey", Accessory: p.S.Hotkey, Icon: "keyboard", Actions: []Action{{Title: "Change…", Run: func(p *Palette) { p.Push(p.hotkeyView()) }}}},
				{Title: "Default Search Engine", Accessory: p.engine().Name, Icon: "search", Actions: []Action{{Title: "Next Engine", Run: func(p *Palette) {
					i := slices.IndexFunc(engines, func(e engine) bool { return e.Key == p.S.Engine })
					p.S.Engine = engines[(i+1)%len(engines)].Key
					p.S.save()
					p.Refresh()
				}}}},
				{Title: "Clipboard History", Accessory: clip, Icon: "clipboard", Actions: []Action{{Title: "Toggle", Run: func(p *Palette) { p.S.ClipOff = !p.S.ClipOff; p.S.save(); p.Refresh() }}}},
				{Title: "Clipboard History Size", Accessory: itoa(p.S.ClipLimit), Icon: "clipboard", Actions: []Action{{Title: "Cycle", Run: func(p *Palette) {
					sizes := []int{50, 200, 500, 1000}
					i := slices.Index(sizes, p.S.ClipLimit)
					p.S.ClipLimit = sizes[(i+1)%len(sizes)]
					p.S.save()
					p.Refresh()
				}}}},
				{Title: "Launch at Login", Accessory: login, Icon: "power", Actions: []Action{{Title: "Toggle", Run: func(p *Palette) { setAutostart(!autostartEnabled()); p.Refresh() }}}},
				{Title: "Account", Accessory: auth, Icon: "user", Actions: []Action{{Title: "Show Auth File", Run: func(p *Palette) { p.Hide(); mygo.Shell.ShowItemInFolder(filepath.Join(configDir(), "auth.json")) }}}},
				{Title: "Open Config Folder", Subtitle: configDir(), Icon: "folder", Actions: []Action{{Title: "Open", Run: func(p *Palette) { p.Hide(); _ = mygo.Shell.OpenPath(configDir()) }}}},
			}
			if lq == "" {
				return rows
			}
			return slices.DeleteFunc(rows, func(it Item) bool { return !strings.Contains(strings.ToLower(it.Title), lq) })
		},
	}
}

func (p *Palette) hotkeyView() *View {
	return &View{
		Title: "Hotkey", Placeholder: "Press keys, e.g. Alt+Shift+Space",
		Search: func(q string) []Item {
			if q == "" {
				return []Item{{Title: "Type a new accelerator", Subtitle: "Current: " + p.S.Hotkey, Icon: "keyboard"}}
			}
			return []Item{{Title: "Use “" + q + "”", Icon: "keyboard", Actions: []Action{{Title: "Save", Run: func(p *Palette) {
				old := p.S.Hotkey
				mygo.GlobalShortcut.Unregister(old)
				if err := mygo.GlobalShortcut.Register(q, p.toggle); err != nil {
					_ = mygo.GlobalShortcut.Register(old, p.toggle)
					p.Toast("Invalid or taken: " + err.Error())
					return
				}
				p.S.Hotkey = q
				p.S.save()
				p.Pop()
				p.Toast("Hotkey set to " + q)
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
func sectioned(items []Item, searching bool) []Item {
	if len(items) == 0 {
		return items
	}
	var top []Item
	rest := items
	if searching {
		top, rest = items[:1], slices.Clone(items[1:])
	} else {
		rest = slices.Clone(items)
	}
	slices.SortStableFunc(rest, func(a, b Item) int { return cmp.Compare(sectionRank(a.Kind), sectionRank(b.Kind)) })
	out := make([]Item, 0, len(items))
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
