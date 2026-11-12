package agent

import (
	"context"
	"fmt"
	"sync"
)

// SubCalls lets a running tool report the actions it takes for the user as calls nested under its own, and ask the user
// to allow one of them while the tool waits. The agent package answers the user through the same approval channel as a
// tool's own confirmation, so the chat shows one kind of question. A tool reads it with SubCallsFrom.
type SubCalls interface {
	// Begin starts a nested call. The transcript shows label while it runs, and detail under it. Detail must be built
	// by the host from what it knows, never copied from the model's words.
	Begin(label, detail string) SubCall
}

// SubCall is one nested call. It ends exactly once, with End.
type SubCall interface {
	// Ask shows the user an approval question for this call and waits for the answer. It returns false when the user
	// declines, and an error when ctx ends first, in which case nothing may run afterwards.
	Ask(ctx context.Context) (allowed bool, err error)
	// End closes the call. result is the text kept with its row, and outcome the one line the transcript shows.
	End(status CallStatus, outcome, result string)
}

type subCallsKey struct{}

// WithSubCalls returns ctx carrying calls. The agent package sets it for every tool run; tests of a tool set it to
// observe the nested calls.
func WithSubCalls(ctx context.Context, calls SubCalls) context.Context {
	return context.WithValue(ctx, subCallsKey{}, calls)
}

// SubCallsFrom returns the nested-call port of the running tool, or nil when the tool runs outside a turn. A tool
// without one must refuse an action that needs the user's approval.
func SubCallsFrom(ctx context.Context) SubCalls {
	calls, _ := ctx.Value(subCallsKey{}).(SubCalls)
	return calls
}

// callScope is the nested calls of one tool call.
type callScope struct {
	run    *turnRun
	parent CallInfo
	mu     sync.Mutex
	next   int
}

func (s *callScope) Begin(label, detail string) SubCall {
	s.mu.Lock()
	s.next++
	id := fmt.Sprintf("%s:%d", s.parent.ID, s.next)
	s.mu.Unlock()
	info := CallInfo{ID: id, ParentID: s.parent.ID, Name: s.parent.Name + ".step", Label: label, Input: detail}
	s.run.emit(Event{Kind: EventToolStart, Call: info})
	return &subCall{run: s.run, info: info}
}

type subCall struct {
	run  *turnRun
	info CallInfo
	once sync.Once
}

func (c *subCall) Ask(ctx context.Context) (bool, error) {
	asked := c.run.setup.Clock.Now()
	allowed, cancelled := c.run.await(ctx, c.info.ID, c.info)
	c.run.waited.Add(int64(c.run.setup.Clock.Now().Sub(asked)))
	if cancelled {
		return false, context.Cause(ctx)
	}
	return allowed, nil
}

func (c *subCall) End(status CallStatus, outcome, result string) {
	c.once.Do(func() {
		c.run.emit(Event{Kind: EventToolEnd, Call: c.info, Status: status, Result: result, Outcome: outcome})
	})
}
