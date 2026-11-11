package actioncli

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMalformedRequestsAreRejectedBeforeDispatch(t *testing.T) {
	called := false
	dispatch := func(context.Context, string) Response { called = true; return Response{Outcome: Completed} }
	for _, raw := range []string{"", "not json\n", `{"v":2,"action":"x"}` + "\n", `{"v":1,"action":""}` + "\n", `{"v":1,"action":"a\u0000b"}` + "\n"} {
		r := answer(context.Background(), strings.NewReader(raw), dispatch)
		if r.Outcome != Rejected {
			t.Errorf("%q: %v", raw, r)
		}
	}
	if called {
		t.Fatal("a malformed request reached the dispatcher")
	}
}

func TestDispatchHasADeadline(t *testing.T) {
	r := answer(context.Background(), strings.NewReader(`{"v":1,"action":"x"}`+"\n"), func(ctx context.Context, _ string) Response {
		if _, ok := ctx.Deadline(); !ok {
			return Response{Outcome: Failed, Message: "no deadline"}
		}
		return Response{Outcome: Completed}
	})
	if r.Outcome != Completed {
		t.Fatal(r.Message)
	}
}

// Concurrent clients over a real socket each run exactly once.
func TestConcurrentRequestsRunOnceEach(t *testing.T) {
	dir, err := os.MkdirTemp("", "nvc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Skip("unix sockets unavailable:", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	counts := map[string]int{}
	var total atomic.Int32
	go Serve(ctx, ln, time.Now, func(_ context.Context, id string) Response {
		mu.Lock()
		counts[id]++
		mu.Unlock()
		total.Add(1)
		return Response{Outcome: Completed}
	})
	dial := func(timeout time.Duration) (net.Conn, error) { return net.DialTimeout("unix", path, timeout) }
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r, err := Forward(dial, time.Now, "action-"+string(rune('a'+i))); err != nil || r.Outcome != Completed {
				t.Errorf("forward: %v %v", r, err)
			}
		}()
	}
	wg.Wait()
	if total.Load() != 8 || len(counts) != 8 {
		t.Fatalf("dispatched %d over %d ids", total.Load(), len(counts))
	}
}
