package devicemodel

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func checksum(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func TestTheEmbeddedManifestPinsEveryFileTheModelNeeds(t *testing.T) {
	manifest := Parakeet()
	if manifest.Name == "" || manifest.Title == "" || manifest.Credit == "" || manifest.BaseURL == "" {
		t.Fatalf("manifest = %+v; want it named, credited and sourced", manifest)
	}
	names := map[string]bool{}
	for _, file := range manifest.Files {
		if file.Size <= 0 || len(file.SHA256) != 64 {
			t.Fatalf("file %+v is not pinned by size and checksum", file)
		}
		names[file.Name] = true
	}
	for _, need := range []string{"parakeet-tdt-0.6b-v3-Q8_0.gguf"} {
		if !names[need] {
			t.Fatalf("the manifest lacks %s", need)
		}
	}
}

func TestInstallDownloadsEachFileOnceAndTheModelIsReady(t *testing.T) {
	root := t.TempDir()
	downloads := fake.NewDownloads()
	downloads.Block = make(chan struct{})
	manifest := Manifest{Name: "tiny", BaseURL: "https://models.test/tiny", Files: []File{
		{Name: "a.txt", Size: 3, SHA256: checksum("abc")},
		{Name: "b.onnx", Size: 5, SHA256: checksum("hello")},
	}}
	downloads.Serve("https://models.test/tiny/a.txt", []byte("abc"))
	downloads.Serve("https://models.test/tiny/b.onnx", []byte("hello"))
	model := New(real.NewPorts().FS, downloads, root, manifest)
	if got := model.State().Status; got != StatusMissing {
		t.Fatalf("status = %s; want missing", got)
	}
	background := func(_ string, fn func()) { go fn() }
	model.Install(background)
	model.Install(background)
	if got := model.State().Status; got != StatusDownloading {
		t.Fatalf("status = %s; want downloading", got)
	}
	close(downloads.Block)
	deadline := time.Now().Add(2 * time.Second)
	for model.State().Status != StatusReady {
		if time.Now().After(deadline) {
			t.Fatalf("state = %+v; want ready", model.State())
		}
		time.Sleep(time.Millisecond)
	}
	if fetched := downloads.Fetched(); len(fetched) != 2 {
		t.Fatalf("fetched %v; want each file once", fetched)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "tiny", "b.onnx")); string(data) != "hello" {
		t.Fatalf("b.onnx = %q", data)
	}
}

func TestAFailedDownloadSaysWhyAndCanBeResumed(t *testing.T) {
	downloads := fake.NewDownloads()
	manifest := Manifest{Name: "tiny", BaseURL: "https://models.test/tiny", Files: []File{{Name: "a.txt", Size: 3, SHA256: checksum("abc")}}}
	model := New(real.NewPorts().FS, downloads, t.TempDir(), manifest)
	model.Install(func(_ string, fn func()) { fn() })
	if state := model.State(); state.Status != StatusFailed || state.Error == "" {
		t.Fatalf("state = %+v; want a failure with its reason", state)
	}
	downloads.Serve("https://models.test/tiny/a.txt", []byte("abc"))
	model.Install(func(_ string, fn func()) { fn() })
	if state := model.State(); state.Status != StatusReady {
		t.Fatalf("state = %+v; want ready after the second try", state)
	}
}
