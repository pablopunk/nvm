package protocol

// EntryPoints maps each method a child can serve to the top-level function of the extension source that implements it.
var EntryPoints = map[string]string{
	MethodCommands: "Commands",
	MethodRoot:     "RootItems",
	MethodSearch:   "SearchItems",
	MethodRun:      "RunAction",
	MethodLoad:     "LoadItems",

	MethodMenuBars:        "MenuBars",
	MethodMenuBarTriggers: "MenuBarTriggers",
	MethodRestoreWindow:   "RestoreWindow",
}
