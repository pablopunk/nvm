package api

// QuickLook shows a file in the OS previewer outside Nevermind. Labels come from System().Labels().PreviewFile.
func QuickLook(title, path string) Action {
	return Action{Kind: ActionQuickLook, Title: title, Path: path}
}

// OpenWith opens a file with one application, by its bundle path. The row's menu lists the apps from
// Files().OpenWithApps.
func OpenWith(title, path, appPath string) Action {
	return Action{Kind: ActionOpenWith, Title: title, Path: path, App: appPath}
}

// Trash moves paths to the Trash. It is destructive and asks first when the action carries a Confirmation.
func Trash(title string, paths ...string) Action {
	return Destructive(Action{Kind: ActionTrash, Title: title, Paths: paths})
}

// PreviewFile is a full-palette preview of a file: media renders natively, text shows its leading 64 KB. The host
// reads the file. Use it as an item's Primary or in its Actions.
func PreviewFile(title string, file FileRef) Action {
	view := View{Kind: ViewFilePreview, Title: file.Name, File: &FilePreview{File: file}}
	return Action{Kind: ActionPushView, Title: title, View: &view}
}

// PreviewFileView is the view PreviewFile pushes, for a view that wants the preview itself.
func PreviewFileView(file FileRef) View {
	return View{Kind: ViewFilePreview, Title: file.Name, File: &FilePreview{File: file}}
}
