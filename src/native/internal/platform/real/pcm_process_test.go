//go:build !windows

package real

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"testing"
)

func TestAClosedStreamEndsWithEOFEvenWhenItsPipeIsClosedUnderARead(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "exec cat /dev/zero")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stream := newPCMProcessStream(cmd, stdout, &stderr, macStreamError)
	pcm := make([]int16, 320)
	if _, err := stream.Read(pcm); err != nil {
		t.Fatalf("Read before Close: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	for range 10_000 {
		_, err := stream.Read(pcm)
		if err == nil {
			continue
		}
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Read after Close = %v, want io.EOF: a stream the dictation closed is not a microphone failure", err)
		}
		return
	}
	t.Fatal("the stream never ended after Close")
}
