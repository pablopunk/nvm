package services

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// realFilesHandler is an extension with desktop.files over the real file system, so walks and reads hit a temp folder.
func realFilesHandler(t *testing.T, deps Deps) *Handler {
	t.Helper()
	deps.DataRoot = t.TempDir()
	deps.Files = real.NewPorts().FS
	deps.Logs = nil
	handler := New(deps, manifestFor("acme.files", api.CapabilityDesktopFiles))
	t.Cleanup(func() { _ = handler.Close() })
	return handler
}

// writePNG writes a width-by-height PNG and returns its path.
func writePNG(t *testing.T, path string, width, height int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// decodeFiles reads the reply data of a find as file refs.
func decodeFiles(t *testing.T, reply protocol.ServiceReply) []api.FileRef {
	t.Helper()
	var refs []api.FileRef
	if err := json.Unmarshal(mustOK(t, reply), &refs); err != nil {
		t.Fatalf("decode refs: %v", err)
	}
	return refs
}

func TestFindReturnsOnlyTheRequestedKindFromExistingRoots(t *testing.T) {
	root := t.TempDir()
	writePNG(t, filepath.Join(root, "a.png"), 4, 4)
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, deps := newDesktopHost(t)
	handler := realFilesHandler(t, deps)
	reply := serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpFind, protocol.FindArgs{
		Roots:   []string{root, filepath.Join(root, "missing")},
		Options: api.FindOptions{Kind: "image", SortBy: "name", Order: "asc", Limit: 10},
	})
	refs := decodeFiles(t, reply)
	if len(refs) != 1 || refs[0].Name != "a.png" || refs[0].Kind != "image" {
		t.Fatalf("refs %+v; want only a.png", refs)
	}
	if !strings.HasPrefix(refs[0].FileURL, "file://") {
		t.Fatalf("fileUrl %q is not a file URL", refs[0].FileURL)
	}
}

func TestFindSkipsRelativeRootsAndRefusesUnknownKinds(t *testing.T) {
	_, deps := newDesktopHost(t)
	handler := realFilesHandler(t, deps)
	if refs := decodeFiles(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpFind, protocol.FindArgs{Roots: []string{"Downloads"}})); len(refs) != 0 {
		t.Fatalf("a relative root searched: %+v", refs)
	}
	requireCode(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpFind, protocol.FindArgs{Roots: []string{t.TempDir()}, Options: api.FindOptions{Kind: "archive"}}), api.CodeInvalid, "kind")
	requireCode(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpFind, protocol.FindArgs{Roots: []string{t.TempDir()}, Options: api.FindOptions{SortBy: "color"}}), api.CodeInvalid, "sort")
}

func TestMetadataDescribesAFileAndItsPictureSize(t *testing.T) {
	root := t.TempDir()
	picture := writePNG(t, filepath.Join(root, "wide.png"), 7, 3)
	_, deps := newDesktopHost(t)
	handler := realFilesHandler(t, deps)
	var ref api.FileRef
	if err := json.Unmarshal(mustOK(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpMetadata, protocol.PathArgs{Path: picture})), &ref); err != nil {
		t.Fatal(err)
	}
	if ref.Kind != "image" || ref.Extension != "png" || ref.Width != 7 || ref.Height != 3 || ref.MimeType != "image/png" {
		t.Fatalf("metadata %+v", ref)
	}
	requireCode(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpMetadata, protocol.PathArgs{Path: filepath.Join(root, "gone.png")}), api.CodeNotFound, "gone.png")
}

func TestThumbnailIsAPNGDataURLOnlyForPictures(t *testing.T) {
	root := t.TempDir()
	picture := writePNG(t, filepath.Join(root, "big.png"), 800, 400)
	text := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(text, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, deps := newDesktopHost(t)
	handler := realFilesHandler(t, deps)
	var thumb protocol.ThumbnailResult
	if err := json.Unmarshal(mustOK(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: picture})), &thumb); err != nil {
		t.Fatal(err)
	}
	if !thumb.Found || !strings.HasPrefix(thumb.URL, pngDataURLPrefix) {
		t.Fatalf("thumbnail %+v; want a PNG data URL", thumb)
	}
	var none protocol.ThumbnailResult
	if err := json.Unmarshal(mustOK(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpThumbnail, protocol.PathArgs{Path: text})), &none); err != nil {
		t.Fatal(err)
	}
	if none.Found || none.URL != "" {
		t.Fatalf("text file thumbnail %+v; want none", none)
	}
}

func TestFileURLEncodesTheAbsolutePath(t *testing.T) {
	_, deps := newDesktopHost(t)
	handler := realFilesHandler(t, deps)
	var link string
	if err := json.Unmarshal(mustOK(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpFileURL, protocol.PathArgs{Path: "/Users/me/my file.png"})), &link); err != nil {
		t.Fatal(err)
	}
	if link != "file:///Users/me/my%20file.png" {
		t.Fatalf("file url %q", link)
	}
}

// toolRunner answers each tool with the output queued for the argument it names, so the parallel scoring of apps
// is deterministic.
type toolRunner struct {
	outputs map[string]string
}

func (r toolRunner) Run(_ context.Context, spec platform.CommandSpec) (platform.HelperResult, error) {
	for needle, output := range r.outputs {
		for _, arg := range spec.Args {
			if strings.Contains(arg, needle) {
				return platform.HelperResult{Stdout: []byte(output)}, nil
			}
		}
	}
	return platform.HelperResult{}, nil
}

func TestOpenWithAppsRanksByDocumentType(t *testing.T) {
	host, deps := newDesktopHost(t)
	host.shell.SetApps([]platform.App{
		{Name: "Safari", ID: "safari", Path: "/Applications/Safari.app"},
		{Name: "Preview", ID: "preview", Path: "/Applications/Preview.app"},
		{Name: "TextEdit", ID: "textedit", Path: "/Applications/TextEdit.app"},
		{Name: "Folder", ID: "folder", Path: "/Applications/Folder"},
	}, nil)
	deps.Commands = toolRunner{outputs: map[string]string{
		"/Applications/Preview.app":  `{"CFBundleDocumentTypes":[{"CFBundleTypeExtensions":["png","jpg"]}]}`,
		"/Applications/Safari.app":   `{"CFBundleDocumentTypes":[{"CFBundleTypeExtensions":["*"]}]}`,
		"/Applications/TextEdit.app": `{"CFBundleDocumentTypes":[{"CFBundleTypeExtensions":["txt"]}]}`,
		"kMDItemContentTypeTree":     `("public.png","public.image")`,
	}}
	handler := realFilesHandler(t, deps)
	var apps []api.App
	if err := json.Unmarshal(mustOK(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpOpenWithApps, protocol.PathArgs{Path: "/Users/me/a.png"})), &apps); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, app.Name)
	}
	if strings.Join(names, ",") != "Preview,Safari" {
		t.Fatalf("apps %v; want Preview then Safari (TextEdit does not open png, Folder is not a bundle)", names)
	}
}

func TestOpenWithAppsListsEveryBundleWhenNoToolAnswers(t *testing.T) {
	host, deps := newDesktopHost(t)
	host.shell.SetApps([]platform.App{
		{Name: "Zed", Path: "/Applications/Zed.app"},
		{Name: "Arc", Path: "/Applications/Arc.app"},
	}, nil)
	handler := realFilesHandler(t, deps)
	var apps []api.App
	if err := json.Unmarshal(mustOK(t, serve(t, handler, protocol.ServiceDesktopFiles, protocol.OpOpenWithApps, protocol.PathArgs{Path: "/Users/me/a.png"})), &apps); err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 || apps[0].Name != "Arc" || apps[1].Name != "Zed" {
		t.Fatalf("apps %+v; want Arc then Zed", apps)
	}
}
