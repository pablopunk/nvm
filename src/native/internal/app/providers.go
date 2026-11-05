package app

import (
	"slices"

	"github.com/pablopunk/nvm/native/internal/shortcuts"

	"github.com/egoist/mygo"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aicommands"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/appcommands"
	placesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/places"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
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
		{ID: shortcuts.CommandAIChats, Title: "AI Chats", Keywords: "ai chat assistant gpt llm ask conversations history", Icon: "sparkles", Tone: "yellow", Subtitle: "Chat with the assistant", Run: func(h search.Host) { h.Push(p.chatHistoryView()) }},

		{ID: shortcuts.CommandExtensions, Title: "Extensions", Keywords: "extensions manage plugins create builder", Icon: "puzzle", Subtitle: "Manage and create extensions", Run: func(h search.Host) { h.Push(p.extensionsView()) }},
	}
}

// trailingCommands are the commands listed after the built-in system commands.
func trailingCommands(p *App) []command {
	cmds := []command{
		{ID: shortcuts.CommandKeyboardShortcuts, Title: "Keyboard Shortcuts", Keywords: "keyboard shortcuts hotkey keybindings keys", Icon: "keyboard", Subtitle: "View, change, or remove shortcuts", Run: func(h search.Host) { h.Push(p.keyboardShortcutsPage()) }},
		{ID: shortcuts.CommandBackgroundTasks, Title: "Background Tasks", Keywords: "background tasks jobs scheduled triggers history failures run", Icon: "activity", Subtitle: "Run, pause, and inspect background work", Run: func(h search.Host) { h.Push(p.backgroundTasksPage()) }},
		{ID: shortcuts.CommandSettings, Title: "Settings", Keywords: "settings preferences config hotkey", Icon: "settings", Subtitle: "Hotkey, search engine, clipboard", Run: func(h search.Host) { h.Push(p.settingsPage()) }},
		{ID: shortcuts.CommandCheckForUpdates, Title: checkForUpdatesTitle, Keywords: "update upgrade version release new", Icon: "rotate-ccw", Subtitle: "Current version: " + mygo.App.Version(), Dismiss: search.DismissNever, Run: func(h search.Host) { p.checkForUpdates(func(string) {}); h.Push(p.updatesPage()) }},
		{ID: shortcuts.CommandShowRecentErrors, Title: "Show Recent Errors", Keywords: "errors diagnostics logs crash failure reference support", Icon: "circle-alert", Subtitle: "Find and copy safe error references", Hidden: true, Run: func(h search.Host) { h.Push(p.diagnosticsPage()) }},
		{ID: shortcuts.CommandReportProblem, Title: "Report a Problem", Keywords: "report problem bug issue support feedback", Icon: "message-circle", Subtitle: "Record a problem reference without screen content", Hidden: true, Run: p.reportProblem},
		builtInSlot(appcommands.ID, 1),
		{ID: shortcuts.CommandOSPermissions, Title: "OS Permissions", Keywords: "os permissions permissions microphone camera privacy", Icon: "shield-check", Subtitle: "Review microphone, camera, and system access", Hidden: true, Run: func(h search.Host) { h.Push(p.permissionsPage()) }},
	}
	cmds = append(cmds, p.accountCommands()...)
	cmds = append(cmds, builtInSlot(placesbuiltin.ID, len(p.systemPlaces())))
	cmds = append(cmds, p.classifyImageSlot()...)
	return cmds
}

// builtInSlot is the placeholder of a built-in's rows in the command list. width is how many Order places they take.
func builtInSlot(id string, width int) command {
	return command{builtIn: id, width: width}
}

func isBuiltInSlot(c command) bool { return c.builtIn != "" }

// commandTones colors the commands the root shows in a tone of their own.
var commandTones = map[string]string{"AI Chats": "purple", "Emoji & Symbols": "yellow", "Floating Notes": "yellow",
	"New Note": "yellow", "New Floating Note": "yellow", "Extensions": "purple", "Toggle Floating Notes": "yellow", aicommands.FixTitle: "purple", aicommands.TriageTitle: "purple"}

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
