package app

import (
	"net/url"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/appcommands"
	placesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/places"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
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

func (p *App) engine() engine {
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
	// ID names the command for shortcut bindings. Commands without a shortcut leave it empty.
	ID                                    string
	Title, Keywords, Icon, Subtitle, Tone string
	// PhraseMatch limits the title match to phrases and word prefixes, so a long list of commands does not answer
	// every query with scattered letters. Keywords still match as aliases.
	PhraseMatch bool
	Run         func(h search.Host)
	Dismiss     search.Dismissal
	Order       int
	Hidden      bool
	// builtIn names the built-in whose rows stand here, and width is how many Order places they take; see commandSlots.
	builtIn string
	width   int
}

// commands lists the palette commands, without the placeholders of built-ins. timerOn tells whether a timer is running,
// which adds the cancel command.
func (p *App) commands(timerOn bool) []command {
	return slices.DeleteFunc(p.commandSlots(timerOn), isBuiltInSlot)
}

// commandSlots lists the palette commands in their tie order. Where a built-in's rows stand, the list holds a
// placeholder (see commandRows). The placeholders take the Order places of the native commands they replaced.
func (p *App) commandSlots(timerOn bool) []command {
	cmds := leadingCommands(p)
	cmds = append(cmds, builtInSlot(systemcommands.ID, systemcommands.SupportedCount(p.systemCapability)))
	cmds = append(cmds, trailingCommands(p, timerOn)...)
	numberCommands(cmds)
	return cmds
}

// leadingCommands are the commands listed before the built-in system commands.
func leadingCommands(p *App) []command {
	return []command{
		{Title: "Clipboard History", Keywords: "clipboard paste copy history", Icon: "clipboard", Tone: "blue", Subtitle: "Browse and paste what you copied", Run: func(h search.Host) { h.Push(p.clipboardView()) }},
		{Title: "AI Chats", Keywords: "ai chat assistant gpt llm ask conversations history", Icon: "sparkles", Tone: "yellow", Subtitle: "Chat with the assistant", Run: func(h search.Host) { h.Push(p.chatHistoryView()) }},
		{Title: "Emoji & Symbols", Keywords: "emoji symbols unicode smiley", Icon: "smile", Subtitle: "Search and paste emoji", Run: func(h search.Host) { h.Push(p.emojiView()) }},
		{Title: "Search Files", Keywords: "files find documents locate", Icon: "folder", Subtitle: "Find files in your home folder", Run: func(h search.Host) { h.Push(p.filesView()) }},
		{Title: "Quick Notes", Keywords: "notes note scratch memo floating", Icon: "sticky-note", Subtitle: "Jot something down", Run: func(h search.Host) { h.Push(p.notesView()) }},
		{Title: "New Note", Keywords: "note create scratch", Icon: "plus", Hidden: true, Run: func(h search.Host) { p.newNote() }},
		{Title: "New Floating Note", Keywords: "note float floating window sticky", Icon: "sticky-note", Hidden: true, Run: func(h search.Host) { h.Hide(); p.newFloatingNote() }},
		{Title: "Dictate", Keywords: "dictate dictation voice speech transcribe", Icon: "mic", Subtitle: "Start or stop cloud voice dictation", Run: func(h search.Host) { h.Hide(); p.dictationFlowFor().Toggle() }},
		{Title: "Snippets", Keywords: "snippets text expand template", Icon: "type", Subtitle: "Saved text you paste often", Run: func(h search.Host) { h.Push(p.snippetsView()) }},
		{Title: "Create Snippet", Keywords: "snippet new add", Icon: "plus", Hidden: true, Run: func(h search.Host) { h.Push(p.snippetForm(nil)) }},
		{Title: "Extensions", Keywords: "extensions manage plugins create builder", Icon: "puzzle", Subtitle: "Manage and create extensions", Run: func(h search.Host) { h.Push(p.extensionsView()) }},
		{Title: "Force Quit Apps", Keywords: "force quit kill processes apps running", Icon: "stop-circle", Subtitle: "Quit or force quit running apps", Run: func(h search.Host) { h.Push(p.forceQuitView()) }},
		{Title: "Quit All Apps", Keywords: "quit all apps close everything", Icon: "power", Subtitle: "Review and quit running apps", Run: func(h search.Host) { h.Push(p.quitAllView()) }},
		{Title: "Create Quicklink", Keywords: "quicklink bookmark url search new", Icon: "link", Hidden: true, Run: func(h search.Host) { h.Push(p.quicklinkForm()) }},
		{Title: "Start Timer", Keywords: "timer countdown alarm", Icon: "timer", Subtitle: "Type e.g. “timer 5m”", Hidden: true, Dismiss: search.DismissNever, Run: func(h search.Host) { h.SetQuery("timer 5m") }},
	}
}

// trailingCommands are the commands listed after the built-in system commands.
func trailingCommands(p *App, timerOn bool) []command {
	cmds := p.meaningFilesCommands()
	cmds = append(cmds, []command{
		{Title: "Keyboard Shortcuts", Keywords: "keyboard shortcuts hotkey keybindings keys", Icon: "keyboard", Subtitle: "View, change, or remove shortcuts", Run: func(h search.Host) { h.Push(p.keyboardShortcutsView()) }},
		{Title: "Background Tasks", Keywords: "background tasks jobs scheduled triggers history failures run", Icon: "activity", Subtitle: "Run, pause, and inspect background work", Run: func(h search.Host) { h.Push(p.backgroundTasksView()) }},
		{Title: "Settings", Keywords: "settings preferences config hotkey", Icon: "settings", Subtitle: "Hotkey, search engine, clipboard", Run: func(h search.Host) { h.Push(p.settingsView()) }},
		{Title: "Check for Updates", Keywords: "update upgrade version release new", Icon: "rotate-ccw", Subtitle: "Look for a newer version of Nevermind", Dismiss: search.DismissNever, Run: func(h search.Host) { p.checkForUpdates(h.Toast) }},
		{Title: "Show Recent Errors", Keywords: "errors diagnostics logs crash failure reference support", Icon: "circle-alert", Subtitle: "Find and copy safe error references", Hidden: true, Run: func(h search.Host) { h.Push(p.diagnosticsView()) }},
		{Title: "Report a Problem", Keywords: "report problem bug issue support feedback", Icon: "message-circle", Subtitle: "Record a problem reference without screen content", Hidden: true, Run: p.reportProblem},
		builtInSlot(appcommands.ID, 1),
		{Title: "OS Permissions", Keywords: "os permissions permissions microphone camera privacy", Icon: "shield-check", Subtitle: "Review microphone, camera, and system access", Hidden: true, Run: func(h search.Host) { h.Push(p.permissionsView()) }},
	}...)
	cmds = append(cmds, builtInSlot(placesbuiltin.ID, len(p.systemPlaces())))
	cmds = append(cmds, p.aiCommandRows()...)
	cmds = append(cmds, p.windowCommands()...)
	if timerOn {
		cmds = append(cmds, command{Title: "Cancel Timer", Keywords: "timer cancel stop", Icon: "timer", Hidden: true, Run: func(h search.Host) { p.cancelTimer(); h.Hide() }})
	}
	return cmds
}

// builtInSlot is the placeholder of a built-in's rows in the command list. width is how many Order places they take.
func builtInSlot(id string, width int) command {
	return command{builtIn: id, width: width}
}

func isBuiltInSlot(c command) bool { return c.builtIn != "" }

// commandTones colors the commands the root shows in a tone of their own.
var commandTones = map[string]string{"Clipboard History": "blue", "AI Chats": "purple", "Emoji & Symbols": "yellow", "Search Files": "blue", "Quick Notes": "yellow",
	"New Note": "yellow", "New Floating Note": "yellow", "Snippets": "teal", "Create Snippet": "teal", "Extensions": "purple", "Force Quit Apps": "red", "Quit All Apps": "red", "Create Quicklink": "blue",
	"Start Timer": "orange", "Cancel Timer": "orange",
	commands.FixTitle: "purple", commands.TriageTitle: "purple", commands.ClassifyTitle: "purple"}

// numberCommands sets each command's Order and tone. A placeholder moves Order on by the places it takes.
func numberCommands(cmds []command) {
	order := 0
	for i := range cmds {
		cmds[i].Order = order
		order += commandSpan(cmds[i])
		if tone, ok := commandTones[cmds[i].Title]; ok {
			cmds[i].Tone = tone
		}
	}
}

func commandSpan(c command) int {
	if isBuiltInSlot(c) {
		return c.width
	}
	return 1
}

// rootViewTitle names the palette's first level, so a refresh can find it among the views pushed on it.
const rootViewTitle = "Nevermind"

// rootView is the palette's first level: the providers of every source, merged by the coordinator.
func (p *App) rootView() *palette.View {
	return &palette.View{Title: rootViewTitle, Providers: p.rootProviders, Hint: "esc Close"}
}

// ---- settings ----

func (p *App) settingsView() *palette.View {
	return &palette.View{
		Title: "Settings", Placeholder: "Search Settings",
		Search: func(q string) []search.Item {
			lq := strings.ToLower(q)
			clip := "On"
			if p.S.ClipOff {
				clip = "Off"
			}
			rows := []search.Item{
				{Title: "Palette Hotkey", Accessory: shortcutLabelOf(p.shortcutsManager().PaletteAccelerator()), Icon: "keyboard", Actions: []search.Action{{Title: "Change…", Run: func(h search.Host) { h.Push(p.paletteRecorderView()) }}}},
				{Title: "Default Search Engine", Accessory: p.engine().Name, Icon: "search", Actions: []search.Action{{Title: "Next Engine", Dismiss: search.DismissNever, Run: func(h search.Host) {
					i := slices.IndexFunc(engines, func(e engine) bool { return e.Key == p.S.Engine })
					p.S.Engine = engines[(i+1)%len(engines)].Key
					p.S.save()
					h.Refresh()
				}}}},
				{Title: "Clipboard History", Accessory: clip, Icon: "clipboard", Actions: []search.Action{{Title: "Toggle", Dismiss: search.DismissNever, Run: func(h search.Host) {
					p.S.ClipOff = !p.S.ClipOff
					p.S.save()
					p.clip.setEnabled(!p.S.ClipOff)
					h.Refresh()
				}}, {Title: "Link Previews", Dismiss: search.DismissNever, Run: func(h search.Host) {
					p.S.ClipNoLinks = !p.S.ClipNoLinks
					p.S.save()
					p.clip.setLinkPreviews(!p.S.ClipNoLinks)
					h.Refresh()
				}}}},
				{Title: "Clipboard History Size", Accessory: itoa(p.S.ClipLimit), Icon: "clipboard", Actions: []search.Action{{Title: "Cycle", Dismiss: search.DismissNever, Run: func(h search.Host) {
					sizes := []int{50, 300, 500, 1000}
					i := slices.Index(sizes, p.S.ClipLimit)
					p.S.ClipLimit = sizes[(i+1)%len(sizes)]
					p.S.save()
					p.clip.setLimit(p.S.ClipLimit)
					h.Refresh()
				}}}},
				{Title: "Dictation", Accessory: deviceTitle(p.dictationFlowFor().Devices(), p.dictationSettings().DeviceID), Icon: "mic", Actions: []search.Action{{Title: "Open", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(p.dictationSettingsView()) }}}},
				{Title: "Send Technical Diagnostics", Subtitle: "Errors, usage and performance, without prompts or clipboard content", Accessory: diagnosticsLabel(p.diagnosticsOff()), Icon: "shield-check", Actions: []search.Action{{Title: "Toggle", Dismiss: search.DismissNever, Run: func(h search.Host) {
					p.toggleDiagnostics()
					h.Refresh()
				}}}},
				{Title: "Updates", Accessory: updateAccessory(p.updateSnapshot().State), Icon: "rotate-ccw", Actions: []search.Action{{Title: "Open", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(p.updatesView()) }}}},
				{Title: "Update Channel", Accessory: p.updateSnapshot().Channel.Label(), Icon: "shield-check", Actions: []search.Action{{Title: "Switch", Dismiss: search.DismissNever, Run: func(h search.Host) {
					if err := p.toggleUpdateChannel(); err != nil {
						h.Toast(err.Error())
					}
					h.Refresh()
				}}}},
				{Title: "Account", Accessory: p.accountSummary(), Icon: "user", Actions: []search.Action{{Title: "Open", Dismiss: search.DismissNever, Run: func(h search.Host) { p.openAccountPage(h) }}}},
				{Title: "Open Config Folder", Subtitle: p.displayPath(sharedDir()), Icon: "folder", Actions: []search.Action{{Title: "Open", Run: func(h search.Host) { h.Hide(); _ = mygo.Shell.OpenPath(sharedDir()) }}}},
			}
			rows = slices.Insert(rows, rowIndexAfter(rows, "Dictation"), p.launchAtLoginRows()...)
			if lq == "" {
				return rows
			}
			return slices.DeleteFunc(rows, func(it search.Item) bool { return !strings.Contains(strings.ToLower(it.Title), lq) })
		},
	}
}

// rowIndexAfter is the position just after the row titled title, or the end when no row has that title, so an
// optional group of rows can follow a named row without counting positions.
func rowIndexAfter(rows []search.Item, title string) int {
	if i := slices.IndexFunc(rows, func(it search.Item) bool { return it.Title == title }); i >= 0 {
		return i + 1
	}
	return len(rows)
}
