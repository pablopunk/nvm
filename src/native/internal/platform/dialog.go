package platform

// FileDialog is the intent "let the user choose files or a folder in the operating system's own dialog". Extension
// form fields of type file, files and folder use it. A canceled dialog returns no paths and no error.
type FileDialog interface {
	// Choose shows the dialog and blocks until the user picks or cancels.
	Choose(req ChooseRequest) ([]string, error)
}

// ChooseRequest configures one file dialog.
type ChooseRequest struct {
	Title       string
	ButtonLabel string
	DefaultPath string
	// Extensions limits the files shown, without leading dots. Empty shows every file.
	Extensions []string
	FilterName string
	// Folder selects directories instead of files.
	Folder bool
	// Multiple allows several entries.
	Multiple bool
	// CreateFolders offers a new-folder button where the OS has one.
	CreateFolders bool
}
