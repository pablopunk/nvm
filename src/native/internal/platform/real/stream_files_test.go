package real

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestStreamPreservesDestinationOnAbortAndOversize(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	denied := errors.New("denied")
	for _, test := range []struct {
		name  string
		input string
		limit int64
		admit func() error
		want  error
	}{
		{"oversize", "long", 3, func() error { return nil }, platform.ErrFileTooLarge},
		{"denied", "new", 3, func() error { return denied }, denied},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := (osFS{}).WriteStream(context.Background(), target, strings.NewReader(test.input), test.limit, true, test.admit)
			if !errors.Is(err, test.want) || result.Published {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "old" {
				t.Fatalf("target=%q err=%v", data, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("staging remains: %v %v", entries, err)
			}
		})
	}
}

func TestStreamExclusivePublicationAndSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	result, err := (osFS{}).WriteStream(context.Background(), target, strings.NewReader("new"), 3, false, func() error { return os.WriteFile(target, []byte("raced"), 0600) })
	if !errors.Is(err, fs.ErrExist) || result.Published {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "raced" {
		t.Fatalf("target=%q", data)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	result, err = (osFS{}).WriteStream(context.Background(), link, strings.NewReader("safe"), 4, true, func() error { return nil })
	if err != nil || !result.Published {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	data, _ = os.ReadFile(target)
	if string(data) != "raced" {
		t.Fatalf("followed link: %q", data)
	}
}

func TestStreamPinsParentAcrossRename(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "parent")
	moved := filepath.Join(root, "moved")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	reader := &heldRealStream{entered: make(chan struct{}), release: make(chan struct{})}
	type outcome struct {
		result platform.StreamFileResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := (osFS{}).WriteStream(context.Background(), filepath.Join(dir, "file"), reader, 3, false, func() error { return nil })
		done <- outcome{result, err}
	}()
	<-reader.entered
	renamed := os.Rename(dir, moved) == nil
	if !renamed && runtime.GOOS != "windows" {
		t.Fatal("renaming the parent of an in-flight write failed")
	}
	if renamed {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	close(reader.release)
	got := <-done
	if got.err != nil || !got.result.Published {
		t.Fatalf("result=%+v err=%v", got.result, got.err)
	}
	// Where the rename succeeded the write must follow the handle to the moved directory and leave the
	// replacement empty; where the OS refuses the rename (Windows) the original directory is the only place
	// the file can be.
	published, other := dir, moved
	if renamed {
		published, other = moved, dir
	}
	if _, err := os.Stat(filepath.Join(other, "file")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("published to the wrong directory: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(published, "file"))
	if err != nil || string(data) != "ok" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

type heldRealStream struct {
	entered, release chan struct{}
	read             bool
}

func (r *heldRealStream) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	close(r.entered)
	<-r.release
	copy(p, "ok")
	return 2, io.EOF
}

func TestStreamCancellationAfterAdmissionPreservesTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result, err := (osFS{}).WriteStream(ctx, target, strings.NewReader("new"), 3, true, func() error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) || result.Published {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "old" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}
