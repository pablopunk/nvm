package real

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failedStreamWriter struct{ err error }

func (w failedStreamWriter) Write([]byte) (int, error) { return 0, w.err }

func TestStreamFilesystemFailuresPreserveTarget(t *testing.T) {
	boom := errors.New("injected")
	tests := []struct {
		name string
		ops  streamFileOps
	}{
		{"write", streamFileOps{writeFile: func(*os.File) io.Writer { return failedStreamWriter{boom} }}},
		{"sync", streamFileOps{syncFile: func(*os.File) error { return boom }}},
		{"publish", streamFileOps{publishFile: func(*os.Root, string, string, bool) error { return boom }}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "file")
			if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := writeStream(context.Background(), target, strings.NewReader("new"), 3, true, func() error { return nil }, test.ops)
			if !errors.Is(err, boom) || result.Published {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			b, err := os.ReadFile(target)
			if err != nil || string(b) != "old" {
				t.Fatalf("file=%q err=%v", b, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("entries=%v err=%v", entries, err)
			}
		})
	}
}

func TestStreamCleanupFailureRetainsReservationUntilRetry(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file")
	boom := errors.New("cleanup")
	attempts := 0
	ops := streamFileOps{removeFile: func(root *os.Root, name string) error {
		attempts++
		if attempts == 1 {
			return boom
		}
		return root.Remove(name)
	}}
	pictureStaging.mu.Lock()
	before := pictureStaging.used
	pictureStaging.mu.Unlock()
	result, err := writeStream(context.Background(), target, strings.NewReader("new"), 3, false, func() error { return nil }, ops)
	if !errors.Is(err, boom) || !result.Published {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	pictureStaging.mu.Lock()
	retained := pictureStaging.used
	pictureStaging.mu.Unlock()
	if retained != before+4 {
		t.Fatalf("reservation: before=%d after=%d", before, retained)
	}
	pictureStaging.retryCleanups()
	pictureStaging.mu.Lock()
	after := pictureStaging.used
	pictureStaging.mu.Unlock()
	if after != before || attempts != 2 {
		t.Fatalf("reservation=%d attempts=%d", after, attempts)
	}
	b, err := os.ReadFile(target)
	if err != nil || string(b) != "new" {
		t.Fatalf("file=%q err=%v", b, err)
	}
}

func TestStreamAbortCleanupFailureRetainsReservation(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file")
	denied := errors.New("denied")
	cleanupError := errors.New("cleanup")
	attempts := 0
	ops := streamFileOps{removeFile: func(root *os.Root, name string) error {
		attempts++
		if attempts == 1 {
			return cleanupError
		}
		return root.Remove(name)
	}}
	pictureStaging.mu.Lock()
	before := pictureStaging.used
	pictureStaging.mu.Unlock()
	result, err := writeStream(context.Background(), target, strings.NewReader("new"), 3, false, func() error { return denied }, ops)
	if !errors.Is(err, denied) || !errors.Is(err, cleanupError) || result.Published {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	pictureStaging.mu.Lock()
	retained := pictureStaging.used
	pictureStaging.mu.Unlock()
	if retained != before+4 {
		t.Fatalf("reservation: before=%d after=%d", before, retained)
	}
	pictureStaging.retryCleanups()
	pictureStaging.mu.Lock()
	after := pictureStaging.used
	pictureStaging.mu.Unlock()
	if after != before || attempts != 2 {
		t.Fatalf("reservation=%d attempts=%d", after, attempts)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
}
