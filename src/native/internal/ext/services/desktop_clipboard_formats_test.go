package services

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakeHandles writes handles into a folder of the test, as the host's picture files do.
type fakeHandles struct{ dir string }

func (f fakeHandles) Handle(png []byte) (string, error) {
	path := filepath.Join(f.dir, "handle.png")
	return path, os.WriteFile(path, png, 0o600)
}

func (f fakeHandles) Path(string) (string, error) { return "", os.ErrNotExist }

func (f fakeHandles) Preview(string) string { return "" }

// testPNG is a small picture, encoded as PNG.
func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.NRGBA{R: 200, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestHTMLWritesMarkupAndItsTextFlavor(t *testing.T) {
	host, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteHTML, protocol.WriteHTMLArgs{HTML: "<b>bold</b>", Text: "bold"}))
	var html string
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpReadHTML, nil)), &html); err != nil || html != "<b>bold</b>" {
		t.Fatalf("read back html %q (%v)", html, err)
	}
	if formats, _ := host.clipboard.Formats(); len(formats) == 0 {
		t.Fatal("the clipboard holds no format after an HTML write")
	}
}

func TestConcealedHTMLIsRefusedBecauseTheClipboardCannotMarkIt(t *testing.T) {
	host, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	requireCode(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteHTML, protocol.WriteHTMLArgs{HTML: "<i>secret</i>", Text: "secret", Concealed: true}), api.CodeUnavailable, "history")
	if html, _ := host.clipboard.ReadHTML(); html != "" {
		t.Fatal("a concealed HTML write reached the clipboard")
	}
}

func TestReadReturnsTheFirstFormatTheClipboardHolds(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteHTML, protocol.WriteHTMLArgs{HTML: "<p>hi</p>", Text: "hi"}))
	var content api.ClipboardContent
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpReadFormats, protocol.FormatsArgs{Formats: []string{"image", "html", "text"}})), &content); err != nil {
		t.Fatal(err)
	}
	if content.Type != "html" || content.HTML != "<p>hi</p>" || content.Text != "hi" {
		t.Fatalf("read %+v, want the html copy", content)
	}
}

func TestReadWithNothingHeldIsEmpty(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	var content api.ClipboardContent
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpReadFormats, nil)), &content); err != nil || content.Type != "empty" {
		t.Fatalf("read %+v (%v), want empty", content, err)
	}
}

func TestImageIsAFileHandleThatWritesBackFromItsPath(t *testing.T) {
	host, deps := newDesktopHost(t)
	deps.ClipboardImages = fakeHandles{dir: t.TempDir()}
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	if err := host.clipboard.WriteImagePNG(testPNG(t)); err != nil {
		t.Fatal(err)
	}
	var result protocol.ImageFileResult
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpReadImageFile, nil)), &result); err != nil || !result.Found {
		t.Fatalf("image file %+v (%v), want a handle", result, err)
	}
	if !filepath.IsAbs(result.File.Path) || result.File.Kind != "image" {
		t.Fatalf("handle %+v is not an absolute image file", result.File)
	}
	if err := host.clipboard.WriteText("replaced"); err != nil {
		t.Fatal(err)
	}
	mustOK(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteImageFile, protocol.WriteImageFileArgs{Path: result.File.Path}))
	if _, err := host.clipboard.ReadImagePNG(); err != nil {
		t.Fatalf("the handle did not come back onto the clipboard: %v", err)
	}
}

func TestFileListWithARelativePathIsRefused(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	requireCode(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteFiles, protocol.WriteFilesArgs{Paths: []string{"notes.txt"}}), api.CodeInvalid, "absolute")
}

func TestWriteContentRejectsAnUnknownType(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilityDesktopClipboard)
	requireCode(t, serve(t, h, protocol.ServiceDesktopClipboard, protocol.OpWriteContent, protocol.ContentArgs{Content: api.ClipboardContent{Type: "rtf"}}), api.CodeInvalid, "rtf")
}
