package pi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// scriptedWorker starts processes that answer each line written to them with the lines respond returns.
type scriptedWorker struct {
	mu      sync.Mutex
	respond func(kind string) []string
	sent    []promptMessage
	starts  int
}

func (w *scriptedWorker) Start(platform.WorkerSpec) (platform.WorkerProcess, error) {
	w.mu.Lock()
	w.starts++
	w.mu.Unlock()
	proc := &scriptedProcess{worker: w, out: make(chan []byte, 16)}
	proc.out <- []byte(`{"type":"ready"}`)
	return proc, nil
}

func (w *scriptedWorker) messages() []promptMessage {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]promptMessage(nil), w.sent...)
}

type scriptedProcess struct {
	worker *scriptedWorker
	out    chan []byte
	once   sync.Once
}

func (p *scriptedProcess) PID() int { return 1 }
func (p *scriptedProcess) WriteLine(line []byte) error {
	var message promptMessage
	_ = json.Unmarshal(line, &message)
	p.worker.mu.Lock()
	p.worker.sent = append(p.worker.sent, message)
	respond := p.worker.respond
	p.worker.mu.Unlock()
	for _, reply := range respond(message.Type) {
		p.out <- []byte(reply)
	}
	return nil
}
func (p *scriptedProcess) ReadLine() ([]byte, error) {
	line, ok := <-p.out
	if !ok {
		return nil, io.EOF
	}
	return line, nil
}
func (p *scriptedProcess) Kill() error {
	p.once.Do(func() { close(p.out) })
	return nil
}
func (p *scriptedProcess) Wait() (int, error) { return 0, nil }
func (p *scriptedProcess) StderrTail() string { return "" }

func newTestEngine(worker platform.Worker, access func() (Access, error)) *Engine {
	return New(Options{
		Worker:  worker,
		Runtime: func() (Runtime, error) { return Runtime{Node: "node", Script: "host.mjs"}, nil },
		Access:  access,
		System:  func() string { return "be brief" },
		Role:    "fast",
	})
}

func signedIn() (Access, error) { return Access{BaseURL: "https://example.test", Token: "t"}, nil }

func TestPrepareReadiesTheSessionWithoutAPrompt(t *testing.T) {
	worker := &scriptedWorker{respond: func(kind string) []string { return []string{`{"type":"prepared"}`} }}
	engine := newTestEngine(worker, signedIn)
	if err := engine.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	sent := worker.messages()
	if len(sent) != 1 || sent[0].Type != "prepare" || sent[0].Text != "" || sent[0].Role != "fast" || sent[0].System != "be brief" {
		t.Fatalf("sent %+v, want one prepare message with the role and instruction and no text", sent)
	}
}

func TestPrepareDoesNothingWhileAReplyRuns(t *testing.T) {
	worker := &scriptedWorker{respond: func(string) []string { return nil }}
	engine := newTestEngine(worker, signedIn)
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if err := engine.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if worker.starts != 0 {
		t.Fatalf("started %d hosts while a reply ran", worker.starts)
	}
}

func TestPrepareSignedOutStartsNothing(t *testing.T) {
	worker := &scriptedWorker{respond: func(string) []string { return nil }}
	engine := newTestEngine(worker, func() (Access, error) { return Access{}, errors.New("sign in") })
	if err := engine.Prepare(context.Background()); err == nil {
		t.Fatal("Prepare succeeded while signed out")
	}
	if worker.starts != 0 {
		t.Fatalf("started %d hosts while signed out", worker.starts)
	}
}

func TestPrepareFailureStaysAnErrorAndLeavesTheHostUsable(t *testing.T) {
	worker := &scriptedWorker{respond: func(string) []string {
		return []string{`{"type":"error","code":"failed","message":"the model request failed (500)."}`}
	}}
	engine := newTestEngine(worker, signedIn)
	err := engine.Prepare(context.Background())
	if err == nil || err.Error() != "the model request failed (500)." {
		t.Fatalf("Prepare error = %v", err)
	}
}

func TestPrepareStopsWhenCancelled(t *testing.T) {
	worker := &scriptedWorker{respond: func(string) []string { return nil }}
	engine := newTestEngine(worker, signedIn)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := engine.Prepare(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Prepare error = %v, want the deadline", err)
	}
	if engine.proc != nil {
		t.Fatal("the host kept running after a cancelled prepare")
	}
}
