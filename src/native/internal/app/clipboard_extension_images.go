package app

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Folder and limits of the pictures extensions see. Handles are the PNG files a read picture is written to. Previews are
// the size a detail pane draws, and the memory keeps a few hundred of them.
const (
	clipHandleDirName  = "clipboard-handles"
	clipPreviewEdge    = 1024
	clipPictureMemoMax = 256
	pngDataURLPrefix   = "data:image/png;base64,"
)

// clipImageFiles is the picture side of the clipboard engine: it names the stored pictures for the extension services,
// writes the pictures extensions read to files, and keeps the thumbnails and previews the rows draw. The history owns
// the stored pictures; this only reads them.
type clipImageFiles struct {
	engine   *clipEngine
	dir      string
	mu       sync.Mutex
	pictures map[string]string
	order    []string
}

// newClipImageFiles returns the picture side of an engine, writing handles under the data folder.
func newClipImageFiles(engine *clipEngine, dataFolder string) *clipImageFiles {
	return &clipImageFiles{engine: engine, dir: filepath.Join(dataFolder, clipHandleDirName), pictures: map[string]string{}}
}

// Handle writes PNG bytes to a file in the handles folder, named by their hash, and returns its path. The same picture
// is written once.
func (c *clipImageFiles) Handle(png []byte) (string, error) {
	sum := sha256.Sum256(png)
	path := filepath.Join(c.dir, hex.EncodeToString(sum[:])+".png")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, png, 0o600)
}

// Path returns the PNG file of a stored picture. The history keeps the file while the entry stays in it.
func (c *clipImageFiles) Path(ref string) (string, error) {
	return c.engine.images.FilePath(ref)
}

// Preview returns the PNG data URL of a stored picture at the size a detail pane draws.
func (c *clipImageFiles) Preview(ref string) string {
	return c.cached("preview:"+ref, func() string { return c.render(ref, clipPreviewEdge) })
}

// render reads a stored picture and shrinks it to edge pixels. A picture that cannot be read renders as "".
func (c *clipImageFiles) render(ref string, edge int) string {
	data, err := c.engine.images.Read(ref)
	if err != nil {
		return ""
	}
	png, err := clipboard.PreviewPNG(data, edge)
	if err != nil {
		return ""
	}
	return pngDataURLPrefix + base64.StdEncoding.EncodeToString(png)
}

// cached returns the data URL under key, rendering it once. The memory holds clipPictureMemoMax of them, and drops the
// oldest first.
func (c *clipImageFiles) cached(key string, render func() string) string {
	c.mu.Lock()
	value, ok := c.pictures[key]
	c.mu.Unlock()
	if ok {
		return value
	}
	value = render()
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.pictures[key]; !ok {
		c.order = append(c.order, key)
	}
	c.pictures[key] = value
	for len(c.order) > clipPictureMemoMax {
		delete(c.pictures, c.order[0])
		c.order = c.order[1:]
	}
	return value
}

// clipFilesProxy gives the extension services the picture side of the engine that is open at the time of the call.
// The services are wired before the engine opens, so the proxy reads the engine when it is asked.
type clipFilesProxy struct {
	p *App
}

// extensionClipboardImages returns the picture side the extension services use.
func (p *App) extensionClipboardImages() clipFilesProxy {
	return clipFilesProxy{p: p}
}

func (x clipFilesProxy) files() *clipImageFiles {
	if x.p.clip == nil {
		return nil
	}
	return x.p.clip.files
}

func (x clipFilesProxy) Handle(png []byte) (string, error) {
	if files := x.files(); files != nil {
		return files.Handle(png)
	}
	return "", platform.ErrUnsupported
}

func (x clipFilesProxy) Path(ref string) (string, error) {
	if files := x.files(); files != nil {
		return files.Path(ref)
	}
	return "", platform.ErrUnsupported
}

func (x clipFilesProxy) Preview(ref string) string {
	if files := x.files(); files != nil {
		return files.Preview(ref)
	}
	return ""
}

// copyImageFile puts a picture file on the clipboard, for copy-image. Only the pictures the app owns can be copied: a
// stored history picture or a handle the app wrote, never an arbitrary file an extension names.
func (p *App) copyImageFile(h search.Host, path string) {
	if !p.isClipboardPicturePath(path) {
		h.Toast("Copy failed: the picture is not in Nevermind.")
		return
	}
	png, err := os.ReadFile(path)
	if err == nil {
		err = p.ports.Clipboard.WriteImagePNG(png)
	}
	if err != nil {
		h.Toast(copyFailureMessage(err))
		return
	}
	h.Hide()
}

// isClipboardPicturePath reports whether path is a PNG inside the history's picture folder or the handles folder.
func (p *App) isClipboardPicturePath(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), ".png") {
		return false
	}
	clean := filepath.Clean(path)
	for _, dir := range []string{filepath.Join(dataDir(), clipImageDirName), filepath.Join(dataDir(), clipHandleDirName)} {
		if strings.HasPrefix(clean, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
