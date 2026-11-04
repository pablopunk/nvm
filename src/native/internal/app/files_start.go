package app

import (
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// startFiles builds the Files feature from the settings and starts it. The index loads its saved copy and scans the
// roots on its own goroutine; the palette is usable before either finishes. It runs at the end of ready, after the
// window and the shortcut exist, so a redraw has a window to draw in.
func (p *App) startFiles() {
	home, _ := os.UserHomeDir()
	ports := p.ports
	svc, err := newFileService(fileDeps{
		Ports:     ports,
		Home:      home,
		Roots:     p.S.FileRoots,
		Ignore:    p.S.FileIgnore,
		IndexPath: filepath.Join(configDir(), "files-index.bin"),
		ThumbDir:  filepath.Join(userCacheDir(home), "nvm-native", "files-thumbs"),
		Clipboard: ports.Clipboard,
		CopyFiles: func(paths ...string) error { return mygo.Clipboard.WriteFiles(paths...) },
		Redraw:    p.redrawPanel,
	})
	if err != nil {
		log.Println("files:", err)
		return
	}
	p.files = svc
	svc.start()
	mygo.App.OnQuit(svc.stop)
}

// redrawPanel asks the window to draw again. Caches call it from their workers when a value arrives, so it is safe from
// any goroutine: the window's Update runs the redraw on the UI thread.
func (p *App) redrawPanel() {
	if p.panel != nil {
		p.panel.Update(func() {})
	}
}

// userCacheDir returns the operating system's cache folder, or a folder under home when it has none.
func userCacheDir(home string) string {
	if dir, err := os.UserCacheDir(); err == nil {
		return dir
	}
	return filepath.Join(home, ".cache")
}
