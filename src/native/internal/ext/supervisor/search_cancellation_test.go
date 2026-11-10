package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type searchProcess struct {
	lines  chan []byte
	writes chan []byte
	killed chan struct{}
	reaped chan struct{}
	once   sync.Once
}

func newSearchProcess() *searchProcess {
	return &searchProcess{lines: make(chan []byte, 4), writes: make(chan []byte, 4), killed: make(chan struct{}), reaped: make(chan struct{})}
}
func (p *searchProcess) PID() int                 { return 1 }
func (p *searchProcess) WriteLine(b []byte) error { p.writes <- append([]byte(nil), b...); return nil }
func (p *searchProcess) ReadLine() ([]byte, error) {
	select {
	case b := <-p.lines:
		return b, nil
	case <-p.killed:
		return nil, io.EOF
	}
}
func (p *searchProcess) Kill() error { p.once.Do(func() { close(p.killed) }); return nil }
func (p *searchProcess) Wait() (int, error) {
	<-p.killed
	close(p.reaped)
	return 0, nil
}
func (p *searchProcess) StderrTail() string { return "" }

type searchWorker struct{ started chan *searchProcess }

func receiveSearch[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(waitTimeout):
		t.Fatal("search cancellation did not reach its synchronization boundary")
		var zero T
		return zero
	}
}

func (w *searchWorker) Start(platform.WorkerSpec) (platform.WorkerProcess, error) {
	p := newSearchProcess()
	p.lines <- []byte(`{"ok":true,"methods":["search","run"]}`)
	w.started <- p
	return p, nil
}
func readSearchRequest(t *testing.T, p *searchProcess) protocol.Request {
	t.Helper()
	var req protocol.Request
	if err := json.Unmarshal(receiveSearch(t, p.writes), &req); err != nil {
		t.Fatal(err)
	}
	return req
}
func replySearch(t *testing.T, p *searchProcess, req protocol.Request, data string) {
	t.Helper()
	b, err := json.Marshal(protocol.Response{ID: req.ID, Data: json.RawMessage(data)})
	if err != nil {
		t.Fatal(err)
	}
	p.lines <- b
}

type pendingLineProcess struct {
	*searchProcess
	read chan struct{}
}

func (p pendingLineProcess) ReadLine() ([]byte, error) {
	close(p.read)
	return []byte("pending retired response"), nil
}

func TestRetiringReaderReapsPendingResponseProcess(t *testing.T) {
	p := pendingLineProcess{searchProcess: newSearchProcess(), read: make(chan struct{})}
	done := make(chan struct{})
	lines := readLines(p, done)
	receiveSearch(t, p.read)
	// No response consumer exists, so only retirement can release the reader's pending send.
	close(done)
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	receiveSearch(t, p.reaped)
	select {
	case _, open := <-lines:
		if open {
			t.Fatal("retired reader published its pending response")
		}
	case <-time.After(waitTimeout):
		t.Fatal("retired reader did not close")
	}
}

func TestCancelledSearchRetiresExchangeAndNextRequestSucceeds(t *testing.T) {
	worker := &searchWorker{started: make(chan *searchProcess, 3)}
	r := New(Config{Clock: fake.NewClock(), Worker: worker}, api.Identity{ID: "search"}, "", nil)
	t.Cleanup(r.Stop)
	r.Start()
	old := receiveSearch(t, worker.started)
	if err := r.WaitFor(context.Background(), func(s Status) bool { return s.State == StateRunning }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	oldDone := make(chan error, 1)
	go func() { _, err := r.Call(ctx, protocol.MethodSearch, "old", nil); oldDone <- err }()
	oldReq := readSearchRequest(t, old)
	cancel()
	if err := receiveSearch(t, oldDone); !errors.Is(err, context.Canceled) {
		t.Fatalf("old search: %v", err)
	}
	receiveSearch(t, old.killed)
	receiveSearch(t, old.reaped)
	// Late data belongs to the retired generation and must never satisfy a new request.
	replySearch(t, old, oldReq, `[{"id":"stale"}]`)
	if got := r.Status(); got.State != StateDormant || got.Failures != 0 || !r.Provides(protocol.MethodSearch) {
		t.Fatalf("after cancellation: %+v", got)
	}
	result := make(chan protocol.Response, 1)
	errs := make(chan error, 1)
	go func() {
		response, err := r.Call(context.Background(), protocol.MethodSearch, "new", nil)
		result <- response
		errs <- err
	}()
	fresh := receiveSearch(t, worker.started)
	req := readSearchRequest(t, fresh)
	if req.Query != "new" || req.ID == oldReq.ID {
		t.Fatalf("new request: %+v", req)
	}
	replySearch(t, fresh, req, `[{"id":"fresh"}]`)
	if response := receiveSearch(t, result); string(response.Data) != `[{"id":"fresh"}]` {
		t.Fatalf("new response: %s", response.Data)
	}
	if err := receiveSearch(t, errs); err != nil {
		t.Fatal(err)
	}
	go func() {
		response, err := r.Call(context.Background(), protocol.MethodRun, "", &api.Invocation{})
		result <- response
		errs <- err
	}()
	req = readSearchRequest(t, fresh)
	replySearch(t, fresh, req, `{}`)
	receiveSearch(t, result)
	if err := receiveSearch(t, errs); err != nil {
		t.Fatal(err)
	}
	r.Stop()
}

type searchContextKey struct{}
type heldSearchService struct {
	entered chan any
	release chan struct{}
}

func (s *heldSearchService) Serve(ctx context.Context, _ protocol.ServiceCall) protocol.ServiceReply {
	s.entered <- ctx.Value(searchContextKey{})
	<-s.release
	return protocol.ServiceReply{}
}

func TestCancelledSearchServiceCannotReplyToReplacement(t *testing.T) {
	worker := &searchWorker{started: make(chan *searchProcess, 2)}
	service := &heldSearchService{entered: make(chan any, 1), release: make(chan struct{})}
	r := New(Config{Clock: fake.NewClock(), Worker: worker}, api.Identity{ID: "search"}, "", service)
	t.Cleanup(r.Stop)
	r.Start()
	old := receiveSearch(t, worker.started)
	if err := r.WaitFor(context.Background(), func(s Status) bool { return s.State == StateRunning }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), searchContextKey{}, "origin"))
	done := make(chan error, 1)
	go func() { _, err := r.Call(ctx, protocol.MethodSearch, "old", nil); done <- err }()
	readSearchRequest(t, old)
	old.lines <- []byte(`{"callId":1,"service":"storage","op":"get"}`)
	if value := receiveSearch(t, service.entered); value != "origin" {
		t.Fatalf("service context value: %v", value)
	}
	cancel()
	if err := receiveSearch(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	receiveSearch(t, old.killed)
	receiveSearch(t, old.reaped)
	result := make(chan protocol.Response, 1)
	errs := make(chan error, 1)
	go func() {
		response, err := r.Call(context.Background(), protocol.MethodSearch, "new", nil)
		result <- response
		errs <- err
	}()
	fresh := receiveSearch(t, worker.started)
	req := readSearchRequest(t, fresh)
	close(service.release)
	replySearch(t, fresh, req, `[]`)
	if response := receiveSearch(t, result); string(response.Data) != `[]` {
		t.Fatalf("response: %s", response.Data)
	}
	if err := receiveSearch(t, errs); err != nil {
		t.Fatal(err)
	}
	select {
	case b := <-fresh.writes:
		t.Fatalf("old service replied to replacement: %s", b)
	default:
	}
	r.Stop()
}
