package thumbs

import (
	"bytes"
	"errors"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func TestConcurrentCacheWrites(t *testing.T) {
	fsys := real.NewPorts().FS
	dir := t.TempDir()
	c, err := Open(fsys, dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	data := pngBytes(t, 64, 64)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.write("same", data); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, err := os.ReadFile(c.pathOf("same"))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("cache content: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files: %v %v", entries, err)
	}
	if c.Total() != int64(len(data)) {
		t.Fatalf("total %d", c.Total())
	}
}

func TestSymlinkSwapNeverReadsTarget(t *testing.T) {
	fsys := real.NewPorts().FS
	dir := t.TempDir()
	p := filepath.Join(dir, "image.png")
	target := filepath.Join(dir, "target.png")
	if err := os.WriteFile(target, pngBytes(t, 32, 32), 0600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			os.Remove(p)
			os.Symlink(target, p)
			os.Remove(p)
			os.WriteFile(p, []byte("invalid"), 0600)
		}
	}()
	for i := 0; i < 500; i++ {
		if _, err := Render(fsys, p, 64); err == nil {
			t.Error("read symlink target")
			break
		}
	}
	wg.Wait()
}

type collisionFS struct {
	platform.FS
	attempts int
}

func (f *collisionFS) CreateExclusive(name string, data []byte, perm fs.FileMode) error {
	f.attempts++
	if f.attempts == 1 {
		return fs.ErrExist
	}
	return f.FS.CreateExclusive(name, data, perm)
}

func TestCacheRetriesTemporaryNameCollision(t *testing.T) {
	f := &collisionFS{FS: real.NewPorts().FS}
	c, err := Open(f, t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.write("key", []byte("data")); err != nil {
		t.Fatal(err)
	}
	if f.attempts != 2 {
		t.Fatalf("attempts = %d", f.attempts)
	}
}

func TestCacheCleansTemporaryFilesOnFailure(t *testing.T) {
	for _, op := range []string{fake.OpSync, fake.OpRename} {
		t.Run(op, func(t *testing.T) {
			f := fake.NewFS()
			c, err := Open(f, "/cache", 0)
			if err != nil {
				t.Fatal(err)
			}
			f.FailNext(op, errors.New("injected failure"))
			if err := c.write("key", []byte("data")); err == nil {
				t.Fatal("write succeeded")
			}
			entries, err := f.ReadDir("/cache")
			if err != nil || len(entries) != 0 {
				t.Fatalf("leftovers: %v %v", entries, err)
			}
		})
	}
}

func TestConcurrentAtomicImageReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.png")
	a, b := pngBytes(t, 32, 32), pngBytes(t, 64, 64)
	if err := os.WriteFile(path, a, 0600); err != nil {
		t.Fatal(err)
	}
	fsys := real.NewPorts().FS
	type replacementResult struct {
		completed int
		err       error
	}
	done := make(chan replacementResult, 1)
	start := make(chan struct{})
	go func() {
		<-start
		completed := 0
		for i := 0; i < 200; i++ {
			data := a
			if i%2 == 0 {
				data = b
			}
			tmp := filepath.Join(dir, "replacement")
			if err := os.WriteFile(tmp, data, 0600); err != nil {
				done <- replacementResult{completed, err}
				return
			}
			if err := os.Rename(tmp, path); err != nil {
				if !replacementBlockedByReader(err) {
					done <- replacementResult{completed, err}
					return
				}
			} else {
				completed++
			}
		}
		done <- replacementResult{completed, nil}
	}()
	close(start)
	for i := 0; i < 200; i++ {
		data, err := Render(fsys, path, 64)
		if err != nil {
			t.Errorf("render during replacement: %v", err)
			break
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil || (img.Bounds().Dx() != 32 && img.Bounds().Dx() != 64) {
			t.Errorf("invalid image during replacement: %v", err)
			break
		}
	}
	result := <-done
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.completed == 0 {
		t.Fatal("no replacements succeeded during the concurrent phase")
	}
	// Once the reader is finished, replacement must succeed even if Windows
	// rejected a concurrent replacement of the open destination.
	tmp := filepath.Join(dir, "final-replacement")
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	data, err := Render(fsys, path, 64)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatalf("final replacement: %v", err)
	}
}
