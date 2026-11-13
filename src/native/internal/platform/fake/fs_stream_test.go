package fake

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestStreamAtomicAdmissionAndFaults(t *testing.T) {
	f := NewFS()
	f.Seed("/dir/file", []byte("old"))
	denied := errors.New("denied")
	tests := []struct {
		input string
		limit int64
		admit func() error
		want  error
	}{
		{"overflow", 2, func() error { return nil }, platform.ErrFileTooLarge},
		{"new", 3, func() error { return denied }, denied},
	}
	for _, test := range tests {
		result, err := f.WriteStream(context.Background(), "/dir/file", strings.NewReader(test.input), test.limit, true, test.admit)
		if !errors.Is(err, test.want) || result.Published {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		data, err := f.ReadFile("/dir/file")
		if err != nil || string(data) != "old" {
			t.Fatalf("data=%q err=%v", data, err)
		}
	}
	result, err := f.WriteStream(context.Background(), "/dir/new", strings.NewReader("new"), 3, false, func() error { f.Seed("/dir/new", []byte("raced")); return nil })
	if !errors.Is(err, fs.ErrExist) || result.Published {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

type heldStreamReader struct {
	entered chan struct{}
	release chan struct{}
}

func (r *heldStreamReader) Read([]byte) (int, error) { close(r.entered); <-r.release; return 0, io.EOF }

func TestStreamCapacityWaitIsCancellable(t *testing.T) {
	f := NewFS()
	if err := f.MkdirAll("/dir", 0700); err != nil {
		t.Fatal(err)
	}
	first := &heldStreamReader{entered: make(chan struct{}), release: make(chan struct{})}
	second := &heldStreamReader{entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 2)
	for i, reader := range []*heldStreamReader{first, second} {
		go func(i int, reader *heldStreamReader) {
			_, err := f.WriteStream(context.Background(), fmt.Sprintf("/dir/%d", i), reader, fakeStreamLimit, false, func() error { return nil })
			done <- err
		}(i, reader)
	}
	<-first.entered
	<-second.entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.WriteStream(ctx, "/dir/third", strings.NewReader(""), 1, false, func() error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("capacity cancel: %v", err)
	}
	close(first.release)
	close(second.release)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.WriteStream(context.Background(), "/dir/third", strings.NewReader("ok"), 2, false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestStreamFailuresKeepExistingTarget(t *testing.T) {
	for _, operation := range []string{OpWriteStream, OpWriteFile, OpSync, OpRename} {
		t.Run(operation, func(t *testing.T) {
			f := NewFS()
			f.Seed("/dir/file", []byte("old"))
			injected := errors.New("injected")
			f.FailNext(operation, injected)
			result, err := f.WriteStream(context.Background(), "/dir/file", strings.NewReader("new"), 3, true, func() error { return nil })
			if !errors.Is(err, injected) || result.Published {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			data, err := f.ReadFile("/dir/file")
			if err != nil || string(data) != "old" {
				t.Fatalf("data=%q err=%v", data, err)
			}
		})
	}
}
