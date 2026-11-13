package api

// ExtensionRename holds new names. An empty field keeps its value.
type ExtensionRename struct {
	Title        string `json:"title,omitempty"`
	Subtitle     string `json:"subtitle,omitempty"`
	CommandID    string `json:"commandId,omitempty"`
	CommandTitle string `json:"commandTitle,omitempty"`
}

// Self is what an extension can change about itself.
type Self interface {
	// Rename fails for built-ins, which have no manifest.
	Rename(change ExtensionRename) error
}
