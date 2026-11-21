package codemode

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Defaults of a run's bounds.
const (
	DefaultMaxServiceCalls = 50
	// DefaultApprovalWait is how long a question waits for the user, the palette's own wait for a destructive call.
	DefaultApprovalWait = services.ConfirmWait
)

// serviceHandler answers service calls: the extension services of the program's synthetic identity.
type serviceHandler interface {
	Serve(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply
}

// gate is the host side of one run's service calls and the security boundary of code mode. It decides, for each call,
// whether it is on the table, how many calls the run has made, whether the user must allow it, and only then lets the
// extension services answer. The extension services stay a second line: they refuse a delete the gate did not allow.
type gate struct {
	rules        map[key]rule
	notes        func() *notes.Store
	clock        platform.Clock
	runCtx       context.Context
	steps        agent.SubCalls
	maxCalls     int
	approvalWait time.Duration

	mu      sync.Mutex
	inner   serviceHandler
	count   int
	allowed *key
	asked   map[string]bool
	entries []entry
	// titles are the note titles the gate read from the store to describe a call, kept so the journal can name a note
	// the call then removed.
	titles map[string]string
}

type gateConfig struct {
	rules        map[key]rule
	notes        func() *notes.Store
	clock        platform.Clock
	runCtx       context.Context
	steps        agent.SubCalls
	maxCalls     int
	approvalWait time.Duration
}

func newGate(cfg gateConfig) *gate {
	if cfg.maxCalls <= 0 {
		cfg.maxCalls = DefaultMaxServiceCalls
	}
	if cfg.approvalWait <= 0 {
		cfg.approvalWait = DefaultApprovalWait
	}
	return &gate{rules: cfg.rules, notes: cfg.notes, clock: cfg.clock, runCtx: cfg.runCtx, steps: cfg.steps,
		maxCalls: cfg.maxCalls, approvalWait: cfg.approvalWait, asked: map[string]bool{}, titles: map[string]string{}}
}

// Serve is the supervisor's service handler for the program.
func (g *gate) Serve(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	k := key{call.Service, call.Op}
	over := g.countCall()
	r, listed := g.rules[k]
	if !listed {
		return g.refuseUnlisted(call)
	}
	if over {
		return g.refuseOverLimit(r, call)
	}
	step := g.begin(r.label, r.describe(g, call))
	if verdict := g.verdict(r, k, step); verdict != outcomeRan {
		return g.finishWithoutRunning(r, call, step, verdict)
	}
	reply := g.execute(ctx, k, call)
	return g.finishRan(r, call, step, reply)
}

// verdict decides whether the call may run: the run must still be live, and a class that asks needs the user's allow.
func (g *gate) verdict(r rule, k key, step agent.SubCall) outcome {
	if g.runCtx.Err() != nil {
		return outcomeCancelled
	}
	switch {
	case r.class.asksEachTime():
		return g.ask(step)
	case r.class.asksOncePerService() && !g.wasAllowed(k.service):
		answer := g.ask(step)
		if answer == outcomeRan {
			g.markAllowed(k.service)
		}
		return answer
	}
	return outcomeRan
}

// wasAllowed reports whether the user already allowed reads of the service in this run. A decline is not remembered:
// the next call asks again.
func (g *gate) wasAllowed(service string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.asked[service]
}

func (g *gate) markAllowed(service string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.asked[service] = true
}

// ask puts the call's question to the user and waits. A chat that cannot ask refuses: a destructive or external call
// never runs without an answer.
func (g *gate) ask(step agent.SubCall) outcome {
	if g.steps == nil {
		return outcomeRefused
	}
	waitCtx, cancel := context.WithTimeout(g.runCtx, g.approvalWait)
	defer cancel()
	allowed, err := step.Ask(waitCtx)
	switch {
	case g.runCtx.Err() != nil:
		return outcomeCancelled
	case err != nil:
		return outcomeUnanswered
	case !allowed:
		return outcomeDeclined
	}
	return outcomeRan
}

// execute lets the extension services answer one call the gate allowed. The allowance is for exactly this call and is
// taken back when it returns, so a bug above fails closed at the service's own confirmation.
func (g *gate) execute(ctx context.Context, k key, call protocol.ServiceCall) protocol.ServiceReply {
	g.mu.Lock()
	g.allowed = &k
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.allowed = nil
		g.mu.Unlock()
	}()
	if ctx.Err() != nil || g.runCtx.Err() != nil {
		return failure(api.CodeCancelled, "the chat was stopped before this call ran")
	}
	return g.inner.Serve(ctx, call)
}

// confirm is the confirmation port of the program's extension services. It says yes only while the gate has allowed a
// call and only once for it, so the service's own question can never be answered by anything but the gate.
func (g *gate) confirm(context.Context, services.Question) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.allowed == nil {
		return false, nil
	}
	g.allowed = nil
	return true, nil
}

// countCall counts a call of the run, listed or not, and reports whether the run is past its limit.
func (g *gate) countCall() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.count++
	return g.count > g.maxCalls
}

// begin starts the nested row of a call. Without a chat to show it, the row is silent.
func (g *gate) begin(label, detail string) agent.SubCall {
	if g.steps == nil {
		return silentStep{}
	}
	return g.steps.Begin(label, detail)
}

func (g *gate) record(e entry) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.entries = append(g.entries, e)
}

// journal returns what the gate recorded, in call order.
func (g *gate) journal() []entry {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]entry(nil), g.entries...)
}

// refuseUnlisted answers a call to an operation with no rule. The program is told which operation, and the user sees a row.
func (g *gate) refuseUnlisted(call protocol.ServiceCall) protocol.ServiceReply {
	name := call.Service + "." + call.Op
	g.begin("Blocked a call", name).End(agent.CallRefused, "Not allowed for programs", name+" is not available to programs")
	g.record(entry{class: ClassRead, outcome: outcomeRefused, line: fmt.Sprintf("refused %s: programs cannot use it", name)})
	return failure(api.CodePermission, fmt.Sprintf("%s is not available to programs", name))
}

func (g *gate) refuseOverLimit(r rule, call protocol.ServiceCall) protocol.ServiceReply {
	message := fmt.Sprintf("a program may make at most %d Nevermind calls; combine them or read less", g.maxCalls)
	g.begin(r.label, "").End(agent.CallRefused, "Too many calls", message)
	g.record(entry{class: r.class, outcome: outcomeRefused, line: fmt.Sprintf("refused %s.%s: %s", call.Service, call.Op, message)})
	return failure(api.CodeQuota, message)
}

// finishWithoutRunning closes the row of a call that did not run and tells the program why.
func (g *gate) finishWithoutRunning(r rule, call protocol.ServiceCall, step agent.SubCall, why outcome) protocol.ServiceReply {
	what := r.describe(g, call)
	var e entry
	var reply protocol.ServiceReply
	switch why {
	case outcomeDeclined:
		step.End(agent.CallRefused, "Refused by you", "The user did not allow this.")
		e = entry{r.class, why, fmt.Sprintf("%s: the user declined, so it was not done", lowerFirst(what))}
		reply = failure(api.CodePermission, "the user declined this action, so it was not done; do not try it again")
	case outcomeUnanswered:
		step.End(agent.CallRefused, "No answer in time", "The user did not answer in time.")
		e = entry{r.class, why, fmt.Sprintf("%s: the user did not answer in time, so it was not done", lowerFirst(what))}
		reply = failure(api.CodePermission, "the user did not answer in time, so it was not done")
	case outcomeRefused:
		step.End(agent.CallRefused, "Needs your answer", "This chat cannot ask you, so it was not run.")
		e = entry{r.class, why, fmt.Sprintf("%s: this chat cannot ask the user, so it was not done", lowerFirst(what))}
		reply = failure(api.CodePermission, "this action needs the user's approval and this chat cannot ask for it")
	default:
		step.End(agent.CallCancelled, "Stopped before it ran", "The chat was stopped.")
		e = entry{r.class, why, fmt.Sprintf("%s: the chat was stopped before it ran", lowerFirst(what))}
		reply = failure(api.CodeCancelled, "the chat was stopped")
	}
	g.record(e)
	return reply
}

// finishRan closes the row of a call the services answered.
func (g *gate) finishRan(r rule, call protocol.ServiceCall, step agent.SubCall, reply protocol.ServiceReply) protocol.ServiceReply {
	if reply.Failure != nil {
		line := fmt.Sprintf("%s failed: %s", lowerFirst(r.describe(g, call)), reply.Failure.Message)
		step.End(agent.CallFailed, "Failed: "+reply.Failure.Message, line)
		g.record(entry{r.class, outcomeFailed, line})
		return reply
	}
	line := r.journal(g, call, reply)
	if line == "" {
		line = lowerFirst(r.label)
	}
	step.End(agent.CallRan, clip(line, 120), line)
	g.record(entry{r.class, outcomeRan, line})
	return reply
}

// failure is a refused or failed reply.
func failure(code api.ErrorCode, message string) protocol.ServiceReply {
	return protocol.ServiceReply{Failure: &api.Failure{Code: code, Message: message}}
}

func lowerFirst(text string) string {
	if text == "" {
		return text
	}
	runes := []rune(text)
	if runes[0] >= 'A' && runes[0] <= 'Z' {
		runes[0] += 'a' - 'A'
	}
	return string(runes)
}

// silentStep is the nested call of a run that has no chat to show it.
type silentStep struct{}

func (silentStep) Ask(context.Context) (bool, error)    { return false, nil }
func (silentStep) End(agent.CallStatus, string, string) {}

// remember keeps the title the gate read for a note id.
func (g *gate) remember(id, title string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.titles[id] = title
}

// remembered returns the title the gate read for a note id, or "" when it read none.
func (g *gate) remembered(id string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.titles[id]
}
