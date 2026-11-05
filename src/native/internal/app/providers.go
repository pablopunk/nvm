package app

import (
	"slices"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/shortcuts"

	"github.com/egoist/mygo"
	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aicommands"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/appcommands"
	placesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/places"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

type command struct {
	// ID is the stable name shortcut bindings refer to. It never changes when the title does, and a visible command
	// needs one to take a shortcut.
	ID string
	// rootScore is the command's score at the empty root when it is not the one of its title, such as a row whose
	// title changes with its state.
	rootScore                             int
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

// commands lists the palette commands, without the placeholders of built-ins.
func (p *App) commands() []command {
	return slices.DeleteFunc(p.commandSlots(), isBuiltInSlot)
}

// commandSlots lists the palette commands in their tie order. Where a built-in's rows stand, the list holds a
// placeholder (see commandRows). The placeholders take the Order places of the native commands they replaced.
func (p *App) commandSlots() []command {
	cmds := leadingCommands(p)
	cmds = append(cmds, builtInSlot(systemcommands.ID, systemcommands.SupportedCount(p.systemCapability)))
	cmds = append(cmds, trailingCommands(p)...)
	numberCommands(cmds)
	return cmds
}

// leadingCommands are the commands listed before the built-in system commands.
func leadingCommands(p *App) []command {
	return []command{
		{ID: shortcuts.CommandClipboardHistory, Title: "Clipboard History", Keywords: "clipboard paste copy history", Icon: "clipboard", Tone: "blue", Subtitle: "Browse and paste what you copied", Run: func(h search.Host) { h.Push(p.clipboardView()) }},
		{ID: shortcuts.CommandAIChats, Title: "AI Chats", Keywords: "ai chat assistant gpt llm ask conversations history", Icon: "sparkles", Tone: "yellow", Subtitle: "Chat with the assistant", Run: func(h search.Host) { h.Push(p.chatHistoryView()) }},
		{ID: shortcuts.CommandSearchFiles, Title: "Search Files", Keywords: "files find documents locate", Icon: "folder", Subtitle: "Find files in your home folder", Run: func(h search.Host) { h.Push(p.filesView()) }},
		{ID: shortcuts.CommandQuickNotes, Title: "Floating Notes", Keywords: "notes note scratch memo floating", Icon: "sticky-note", Subtitle: "Jot something down", Run: func(h search.Host) { h.Push(p.notesView()) }},
		{Title: "Toggle Floating Notes", Keywords: "notes note float floating window sticky toggle", Icon: "sticky-note", Subtitle: "Show or hide your floating Markdown notes", Hidden: true, Run: func(h search.Host) { h.Hide(); p.toggleFloatingNotes() }},
		{Title: "New Note", Keywords: "note create scratch", Icon: "plus", Hidden: true, Run: func(h search.Host) { p.newNote() }},
		{ID: shortcuts.CommandNewFloatingNote, Title: "New Floating Note", Keywords: "note float floating window sticky", Icon: "sticky-note", Hidden: true, Run: func(h search.Host) { h.Hide(); p.newFloatingNote() }},
		{ID: shortcuts.CommandDictate, Title: "Dictate", Keywords: "dictate dictation voice speech transcribe", Icon: "mic", Subtitle: "Start or stop cloud voice dictation", Run: func(h search.Host) { h.Hide(); p.dictationFlowFor().Toggle() }},
		{ID: shortcuts.CommandDictationHistory, Title: "Dictation History", Keywords: "dictation history transcripts voice history", Icon: "history", Subtitle: "Browse and copy previous dictations", Run: func(h search.Host) { h.Push(p.dictationHistoryView()) }},
		{ID: shortcuts.CommandExtensions, Title: "Extensions", Keywords: "extensions manage plugins create builder", Icon: "puzzle", Subtitle: "Manage and create extensions", Run: func(h search.Host) { h.Push(p.extensionsView()) }},
	}
}

// trailingCommands are the commands listed after the built-in system commands.
func trailingCommands(p *App) []command {
	cmds := p.meaningFilesCommands()
	cmds = append(cmds, []command{
		{ID: shortcuts.CommandKeyboardShortcuts, Title: "Keyboard Shortcuts", Keywords: "keyboard shortcuts hotkey keybindings keys", Icon: "keyboard", Subtitle: "View, change, or remove shortcuts", Run: func(h search.Host) { h.Push(p.keyboardShortcutsView()) }},
		{ID: shortcuts.CommandBackgroundTasks, Title: "Background Tasks", Keywords: "background tasks jobs scheduled triggers history failures run", Icon: "activity", Subtitle: "Run, pause, and inspect background work", Run: func(h search.Host) { h.Push(p.backgroundTasksView()) }},
		{ID: shortcuts.CommandSettings, Title: "Settings", Keywords: "settings preferences config hotkey", Icon: "settings", Subtitle: "Hotkey, search engine, clipboard", Run: func(h search.Host) { h.Push(p.settingsView()) }},
		{ID: shortcuts.CommandCheckForUpdates, Title: checkForUpdatesTitle, Keywords: "update upgrade version release new", Icon: "rotate-ccw", Subtitle: "Current version: " + mygo.App.Version(), Dismiss: search.DismissNever, Run: func(h search.Host) { p.checkForUpdates(h.Toast) }},
		{ID: shortcuts.CommandShowRecentErrors, Title: "Show Recent Errors", Keywords: "errors diagnostics logs crash failure reference support", Icon: "circle-alert", Subtitle: "Find and copy safe error references", Hidden: true, Run: func(h search.Host) { h.Push(p.diagnosticsView()) }},
		{ID: shortcuts.CommandReportProblem, Title: "Report a Problem", Keywords: "report problem bug issue support feedback", Icon: "message-circle", Subtitle: "Record a problem reference without screen content", Hidden: true, Run: p.reportProblem},
		builtInSlot(appcommands.ID, 1),
		{ID: shortcuts.CommandOSPermissions, Title: "OS Permissions", Keywords: "os permissions permissions microphone camera privacy", Icon: "shield-check", Subtitle: "Review microphone, camera, and system access", Hidden: true, Run: func(h search.Host) { h.Push(p.permissionsView()) }},
	}...)
	cmds = append(cmds, p.accountCommands()...)
	cmds = append(cmds, builtInSlot(placesbuiltin.ID, len(p.systemPlaces())))
	cmds = append(cmds, p.aiCommandRows()...)
	cmds = append(cmds, p.classifyImageSlot()...)
	cmds = append(cmds, p.windowCommands()...)
	return cmds
}

// builtInSlot is the placeholder of a built-in's rows in the command list. width is how many Order places they take.
func builtInSlot(id string, width int) command {
	return command{builtIn: id, width: width}
}

func isBuiltInSlot(c command) bool { return c.builtIn != "" }

// commandTones colors the commands the root shows in a tone of their own.
var commandTones = map[string]string{"Clipboard History": "blue", "AI Chats": "purple", "Emoji & Symbols": "yellow", "Search Files": "blue", "Floating Notes": "yellow",
	"New Note": "yellow", "New Floating Note": "yellow", "Extensions": "purple", "Toggle Floating Notes": "yellow", aicommands.FixTitle: "purple", commands.TriageTitle: "purple"}

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

// rootRowLimit is how many rows the root keeps, as 1.x's SEARCH_RESULT_LIMIT.
const rootRowLimit = 30

// rootEmptyTitle is what the root says when it has no rows, for an empty query or a typed one, as 1.x's root list does.
const rootEmptyTitle = "Type anything"

// rootView is the palette's first level: the providers of every source, merged by the coordinator. Its rows are in
// score order with no section labels, as 1.x's root list shows them.
func (p *App) rootView() *palette.View {
	return &palette.View{Title: rootViewTitle, Providers: p.rootProviders, Hint: "esc Close", Limit: rootRowLimit, Flat: true, EmptyTitle: rootEmptyTitle}
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
				{Title: paletteSettingTitle, Subtitle: paletteSettingSubtitle, Accessory: p.paletteSettingAccessory(), Icon: "keyboard", Actions: []search.Action{{Title: "Change…", Run: func(h search.Host) { h.Push(p.paletteRecorderView()) }}}},
				{Title: hyperKeyTitle, Subtitle: hyperKeySubtitle, Icon: "command", Actions: []search.Action{{Title: "Change…", Run: func(h search.Host) { h.Push(p.hyperRecorderView()) }}}},
				{Title: "Default Search Engine", Accessory: websearch.EngineName(p.webEngineKey()), Icon: "search", Actions: []search.Action{{Title: "Next Engine", Dismiss: search.DismissNever, Run: func(h search.Host) {
					if err := p.cycleWebEngine(); err != nil {
						h.Toast(err.Error())
					}
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
				{Title: "Clipboard History Size", Accessory: strconv.Itoa(p.S.ClipLimit), Icon: "clipboard", Actions: []search.Action{{Title: "Cycle", Dismiss: search.DismissNever, Run: func(h search.Host) {
					sizes := []int{50, 300, 500, 1000}
					i := slices.Index(sizes, p.S.ClipLimit)
					p.S.ClipLimit = sizes[(i+1)%len(sizes)]
					p.S.save()
					p.clip.setLimit(p.S.ClipLimit)
					h.Refresh()
				}}}},
				{Title: "Show Clipboard Items in Root List", Subtitle: "Show copied items inline in the root list", Accessory: onOff(!p.S.ClipRootOff), Icon: "clipboard", Actions: []search.Action{{Title: "Toggle", Dismiss: search.DismissNever, Run: func(h search.Host) {
					p.S.ClipRootOff = !p.S.ClipRootOff
					p.S.save()
					h.Refresh()
				}}}},
				{Title: "Show Clipboard Items in Root Search", Subtitle: "Show copied items when searching from the root list", Accessory: onOff(!p.S.ClipRootSearchOff), Icon: "clipboard", Actions: []search.Action{{Title: "Toggle", Dismiss: search.DismissNever, Run: func(h search.Host) {
					p.S.ClipRootSearchOff = !p.S.ClipRootSearchOff
					p.S.save()
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
