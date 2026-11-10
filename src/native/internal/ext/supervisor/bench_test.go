package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func benchRuntime(b *testing.B) *Runtime {
	b.Helper()
	exe, err := os.Executable()
	if err != nil {
		b.Fatal(err)
	}
	source, err := filepath.Abs(filepath.Join("..", "testdata", "extensions", "acme.good", "main.go"))
	if err != nil {
		b.Fatal(err)
	}
	cfg := Config{Worker: real.NewWorker(), Clock: real.NewPorts().Clock, Executable: exe, StartTimeout: 10 * time.Second}
	return New(cfg, api.Identity{ID: "acme.good", Title: "good"}, source, nil)
}

// BenchmarkDormantFirstCall measures what a user waits for the first time they run a dormant extension.
func BenchmarkDormantFirstCall(b *testing.B) {
	methods := []string{protocol.MethodCommands, protocol.MethodRun}
	for b.Loop() {
		r := benchRuntime(b)
		r.StartDormant(methods)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if _, err := r.Call(ctx, protocol.MethodCommands, "", nil); err != nil {
			b.Fatal(err)
		}
		cancel()
		r.Stop()
	}
}

// BenchmarkSearchRoundTrip measures one search call to a running child, the per-keystroke cost of an extension.
func BenchmarkSearchRoundTrip(b *testing.B) {
	r := benchRuntime(b)
	defer r.Stop()
	r.Start()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := r.WaitFor(ctx, func(s Status) bool { return s.State == StateRunning }); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := r.Call(ctx, protocol.MethodSearch, "hello", nil); err != nil {
			b.Fatal(err)
		}
	}
}
