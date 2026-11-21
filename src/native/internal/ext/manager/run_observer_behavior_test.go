package manager

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

type runCallOverride struct {
	runner
	call func(context.Context, string, string, *api.Invocation) (protocol.Response, error)
}

func (r runCallOverride) Call(ctx context.Context, method, query string, invocation *api.Invocation) (protocol.Response, error) {
	if method == protocol.MethodRun {
		return r.call(ctx, method, query, invocation)
	}
	return r.runner.Call(ctx, method, query, invocation)
}

type threadContextKey struct{}

type outcomeRecorder struct {
	mu  sync.Mutex
	got []RunOutcome
}

func (r *outcomeRecorder) record(outcome RunOutcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, outcome)
}

func (r *outcomeRecorder) outcomes() []RunOutcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]RunOutcome(nil), r.got...)
}

func TestRunObserverWaitsForDecodedResultAndReceivesContext(t *testing.T) {
	recorder := &outcomeRecorder{}
	observer := recordingRunObserver{record: recorder.record}
	h := newHarness(t, func(opts *Options) { opts.RunObserver = observer })
	if err := h.m.Install("acme.settings", fixtureContents(t, "acme.settings"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.settings")
	entered, release := make(chan struct{}), make(chan struct{})
	h.m.mu.Lock()
	e := h.m.entries["acme.settings"]
	base := e.runtime
	e.runtime = runCallOverride{runner: base, call: func(ctx context.Context, _ string, _ string, _ *api.Invocation) (protocol.Response, error) {
		if ctx.Value(threadContextKey{}) != "preserved" {
			return protocol.Response{}, errors.New("derived context value missing")
		}
		close(entered)
		<-release
		return protocol.Response{Data: []byte(`{"toast":{"message":"unchanged"}}`)}, nil
	}}
	h.m.mu.Unlock()
	ctx := context.WithValue(context.Background(), threadContextKey{}, "preserved")
	done := make(chan struct{})
	var result api.Result
	var failure *api.Failure
	var err error
	go func() { result, failure, err = h.m.Run(ctx, "acme.settings", api.Invocation{}); close(done) }()
	<-entered
	if got := recorder.outcomes(); len(got) != 0 {
		t.Fatalf("outcomes before completion = %v", got)
	}
	close(release)
	<-done
	if err != nil || failure != nil || result.Toast == nil || result.Toast.Message != "unchanged" {
		t.Fatalf("Run = %#v, %#v, %v", result, failure, err)
	}
	if got := recorder.outcomes(); len(got) != 1 || got[0] != RunOutcomeSuccess {
		t.Fatalf("outcomes = %v", got)
	}
}

func TestRunObserverClassifiesRunFailures(t *testing.T) {
	tests := []struct {
		name     string
		response protocol.Response
		err      error
		want     RunOutcome
	}{
		{"cancelled", protocol.Response{}, context.Canceled, RunOutcomeCancelled},
		{"deadline", protocol.Response{}, context.DeadlineExceeded, RunOutcomeTimedOut},
		{"call cancelled", protocol.Response{}, &supervisor.CallError{Failure: api.Failure{Code: api.CodeCancelled}}, RunOutcomeCancelled},
		{"call timeout", protocol.Response{}, &supervisor.CallError{Failure: api.Failure{Code: api.CodeTimeout}}, RunOutcomeTimedOut},
		{"ordinary error", protocol.Response{}, errors.New("original"), RunOutcomeFailed},
		{"api cancelled", protocol.Response{Failure: &api.Failure{Code: api.CodeCancelled}}, nil, RunOutcomeCancelled},
		{"api timeout", protocol.Response{Failure: &api.Failure{Code: api.CodeTimeout}}, nil, RunOutcomeTimedOut},
		{"api failure", protocol.Response{Failure: &api.Failure{Code: api.CodeInternal, Message: "same"}}, nil, RunOutcomeFailed},
		{"malformed result", protocol.Response{Data: []byte("{")}, nil, RunOutcomeFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &outcomeRecorder{}
			h := newHarness(t, func(opts *Options) { opts.RunObserver = recordingRunObserver{record: recorder.record} })
			if err := h.m.Install("acme.settings", fixtureContents(t, "acme.settings"), true); err != nil {
				t.Fatal(err)
			}
			h.waitRunning("acme.settings")
			h.m.mu.Lock()
			e := h.m.entries["acme.settings"]
			base := e.runtime
			e.runtime = runCallOverride{runner: base, call: func(context.Context, string, string, *api.Invocation) (protocol.Response, error) {
				return tt.response, tt.err
			}}
			h.m.mu.Unlock()
			_, failure, err := h.m.Run(context.Background(), "acme.settings", api.Invocation{})
			if err != nil {
				t.Fatalf("Run error = %v", err)
			}
			if tt.err != nil {
				if failure == nil {
					t.Fatal("missing mapped failure")
				}
			} else if tt.response.Failure != nil && failure != tt.response.Failure {
				t.Fatalf("failure pointer changed: %p != %p", failure, tt.response.Failure)
			}
			if got := recorder.outcomes(); len(got) != 1 || got[0] != tt.want {
				t.Fatalf("outcomes = %v, want [%s]", got, tt.want)
			}
		})
	}
}

func TestRunObserverLookupErrorAndNilObserver(t *testing.T) {
	recorder := &outcomeRecorder{}
	h := newHarness(t, func(opts *Options) { opts.RunObserver = recordingRunObserver{record: recorder.record} })
	_, _, err := h.m.Run(context.Background(), "missing.extension", api.Invocation{})
	if err == nil || len(recorder.outcomes()) != 1 || recorder.outcomes()[0] != RunOutcomeFailed {
		t.Fatalf("lookup: err %v outcomes %v", err, recorder.outcomes())
	}
	h.m.runObserver = nil
	if _, _, err := h.m.Run(context.Background(), "missing.extension", api.Invocation{}); err == nil {
		t.Fatal("nil observer changed lookup behavior")
	}
}

func TestRunObserverReportsPanicAndPreservesPanic(t *testing.T) {
	recorder := &outcomeRecorder{}
	h := newHarness(t, func(opts *Options) { opts.RunObserver = recordingRunObserver{record: recorder.record} })
	if err := h.m.Install("acme.settings", fixtureContents(t, "acme.settings"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.settings")
	h.m.mu.Lock()
	e := h.m.entries["acme.settings"]
	base := e.runtime
	e.runtime = runCallOverride{runner: base, call: func(context.Context, string, string, *api.Invocation) (protocol.Response, error) {
		panic("original panic")
	}}
	h.m.mu.Unlock()
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_, _, _ = h.m.Run(context.Background(), "acme.settings", api.Invocation{})
	}()
	if recovered != "original panic" {
		t.Fatalf("panic = %v", recovered)
	}
	if got := recorder.outcomes(); len(got) != 1 || got[0] != RunOutcomeFailed {
		t.Fatalf("outcomes = %v", got)
	}
}

func TestRunObserverBeginRunNilFinishAndNilObserver(t *testing.T) {
	h := newHarness(t)
	observer := nilFinishObserver{}
	h.m.runObserver = observer
	_, _, err := h.m.Run(context.Background(), "missing.extension", api.Invocation{})
	if err == nil {
		t.Fatal("expected lookup error")
	}
}

type nilFinishObserver struct{}

func (nilFinishObserver) BeginRun(ctx context.Context) (context.Context, func(RunOutcome)) {
	return ctx, nil
}
