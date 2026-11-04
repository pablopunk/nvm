package settings

// GuideAction is what an item of the guide does when the user presses Enter on it.
type GuideAction string

// The guide's actions. A row without one is information only.
const (
	GuideNone        GuideAction = ""
	GuideSetShortcut GuideAction = "set-shortcut"
	GuideFinish      GuideAction = "finish"
)

// GuideRow is one item of the Getting Started guide.
type GuideRow struct {
	ID       string
	Title    string
	Subtitle string
	Icon     string
	Action   GuideAction
}

// welcomeRootScore puts Welcome above every command suggestion, so a first run starts on it.
const welcomeRootScore = 200

// helpRootScore keeps Help below the suggestions once the guide is done.
const helpRootScore = 14

// GuideEntry is the root row that opens the guide: Welcome until the user is done with it, Help afterwards.
func GuideEntry(completed bool) (title, subtitle string, score int) {
	if completed {
		return "Help", "Quick tips for finding and running commands", helpRootScore
	}
	return "Welcome to Nevermind", "Search apps and files, then press Enter. Launch the app to open the palette or set an optional shortcut.", welcomeRootScore
}

// GuideRows lists the guide's items. shortcut is the palette shortcut the user set, or empty.
func GuideRows(shortcut string) []GuideRow {
	return []GuideRow{
		{ID: "getting-started-find", Title: "Find a command", Subtitle: "Return to the palette, then search for an app, file, or command.", Icon: "search"},
		{ID: "getting-started-run", Title: "Run it", Subtitle: `Choose a result and press Enter. Try "Calculator" or "Open Downloads".`, Icon: "keyboard"},
		shortcutRow(shortcut),
		{ID: "getting-started-permissions", Title: "Permissions when needed", Subtitle: "A prompt appears only when a command needs access.", Icon: "lock"},
		{ID: "getting-started-done", Title: "Done for now", Subtitle: "Dismiss this guide and keep using the palette.", Icon: "check", Action: GuideFinish},
	}
}

// shortcutRow is the guide item about the optional palette shortcut: what it does, and the action to set it.
func shortcutRow(shortcut string) GuideRow {
	row := GuideRow{ID: "getting-started-shortcut", Icon: "keyboard", Action: GuideSetShortcut}
	if shortcut == "" {
		row.Title = "Open Nevermind from anywhere"
		row.Subtitle = "Optional: set a shortcut for instant access. Launching Nevermind from its app icon also opens the palette."
		return row
	}
	row.Title = "Keyboard shortcut set"
	row.Subtitle = shortcut + " opens Nevermind from anywhere. You can change it any time."
	return row
}
