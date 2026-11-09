package api

// ExtensionRename changes how an extension or one of its commands is shown. An empty field keeps its current value.
type ExtensionRename struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	// CommandID names the command CommandTitle applies to.
	CommandID    string `json:"commandId,omitempty"`
	CommandTitle string `json:"commandTitle,omitempty"`
}

// Self is what an extension can change about itself.
type Self interface {
	// Rename writes the new names to the extension's manifest, then the host reloads its rows. Built-in extensions have
	// no manifest to change, so they get an error.
	Rename(change ExtensionRename) error
}
