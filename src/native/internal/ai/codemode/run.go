package codemode

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/program"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Identity is who a conversation's programs are to the host services. It is fixed here: the model cannot choose it, a
// user cannot install it, and its capabilities are the whole of what a program can ask the services for.
var Identity = api.Identity{ID: "nevermind.assistant.program", Title: "Assistant program"}

// capabilities are the areas of the extension api a program may reach. Everything else, including secrets, network,
// files, apps, the shell, the clipboard and system settings, is absent: the services refuse it, and the gate has no rule
// for it either.
var capabilities = []api.Capability{api.CapabilityNotes}

// Options is what a Runner runs on.
type Options struct {
	// Supervisor configures the throwaway child: the worker, clock, executable, memory limits and the budget of
	// protocol.MethodProgram, which counts the program's own computing and not the host's answers to its calls.
	Supervisor supervisor.Config
	// Files stages the program source. StageRoot is the folder each run makes its own folder in.
	Files     platform.FS
	StageRoot string
	// Notes is the user's notes, which the notes service answers from. It returns nil while they are not open.
	Notes func() *notes.Store
	// MaxServiceCalls and ApprovalWait bound a run; zero selects DefaultMaxServiceCalls and DefaultApprovalWait.
	MaxServiceCalls int
	ApprovalWait    time.Duration
}

// Runner runs the programs of conversations.
type Runner struct {
	opts Options
}

// NewRunner returns a runner over opts.
func NewRunner(opts Options) *Runner {
	return &Runner{opts: opts}
}

// Run checks and runs one program body and returns what the model reads. A program that is refused, does not compile,
// fails or is stopped for a bound is a result the model can act on, not an error; an error means the chat itself was
// stopped or the host could not stage the program.
func (r *Runner) Run(ctx context.Context, body string) (agent.Result, error) {
	prepared, err := program.Prepare(body)
	var rejection *program.Rejection
	if errors.As(err, &rejection) {
		return failed("The program was not run: "+rejection.Message, "Not run: "+clip(rejection.Message, 100)), nil
	}
	if err != nil {
		return agent.Result{}, err
	}
	staged, err := stage(r.opts.Files, r.opts.Supervisor.Clock, r.opts.StageRoot, prepared.Source)
	if err != nil {
		return agent.Result{}, err
	}
	defer staged.remove()
	g := newGate(gateConfig{
		rules: notesRules(), notes: r.opts.Notes, clock: r.opts.Supervisor.Clock, runCtx: ctx, steps: agent.SubCallsFrom(ctx),
		maxCalls: r.opts.MaxServiceCalls, approvalWait: r.opts.ApprovalWait,
	})
	handler := services.New(services.Deps{Clock: r.opts.Supervisor.Clock, Notes: r.opts.Notes, Confirm: g.confirm},
		api.Manifest{Identity: Identity, Capabilities: capabilities})
	defer func() { _ = handler.Close() }()
	g.inner = handler
	runtime := supervisor.NewProgram(r.opts.Supervisor, Identity, staged.path, g)
	defer runtime.Stop()
	stopOnCancel := context.AfterFunc(ctx, runtime.Stop)
	defer stopOnCancel()
	runtime.Start()
	response, err := runtime.Do(ctx, protocol.Request{Method: protocol.MethodProgram})
	if ctx.Err() != nil {
		return agent.Result{}, ctx.Err()
	}
	return resultOf(runtime, g.journal(), response, err), nil
}

// resultOf is what the model reads for a finished run, from the child's response or from the way it stopped.
func resultOf(runtime *supervisor.Runtime, entries []entry, response protocol.Response, err error) agent.Result {
	effects := "\n" + journalText(entries)
	var callErr *supervisor.CallError
	switch {
	case errors.As(err, &callErr):
		return stoppedResult(runtime, callErr.Failure, entries, effects)
	case err != nil:
		return failed("The program could not run: "+err.Error()+effects, "Could not run")
	case response.Failure != nil:
		return failed("The program returned an error: "+response.Failure.Message+interpreterHint(response.Failure.Message)+effects, "Failed: "+clip(response.Failure.Message, 100))
	}
	value := string(response.Data)
	if value == "" {
		value = "null"
	}
	return agent.Result{Text: "Result: " + value + effects, Outcome: summary(entries)}
}

// stoppedResult explains a run the host ended or that did not load: it did not compile, ran past its time, used too
// much memory or died.
func stoppedResult(runtime *supervisor.Runtime, failure api.Failure, entries []entry, effects string) agent.Result {
	if rest, compile := strings.CutPrefix(failure.Message, "does not load: "); compile {
		rest = bodyLines(strings.TrimPrefix(rest, "the program does not compile: "))
		return failed("The program did not compile: "+rest+"\nNothing ran.", "Did not compile: "+clip(rest, 100))
	}
	reason := failure.Message
	if status := runtime.Status(); status.Reason != "" && strings.Contains(status.Reason, "memory") {
		reason = status.Reason
	}
	switch failure.Code {
	case api.CodeTimeout:
		return failed(fmt.Sprintf("The program was stopped: it computed for more than its limit (%s). Calls already made are listed.%s", reason, effects), "Stopped: too slow")
	case api.CodeQuota:
		return failed("The program was stopped: "+reason+". Use less memory."+effects, "Stopped: too much memory")
	}
	return failed("The program stopped unexpectedly: "+reason+effects, "Stopped: "+clip(reason, 100))
}

func failed(text, outcome string) agent.Result {
	return agent.Result{Text: text, Outcome: outcome, Failed: true}
}

// interpreterHint explains an interpreter limit that reads as a mystery: a panic the program did not cause.
func interpreterHint(message string) string {
	if strings.Contains(message, "SetBool on interface") {
		return "\n(The interpreter cannot return a computed bool directly. Write ok := a < b, then return ok, nil.)"
	}
	return ""
}

// sourcePosition matches the line:column the interpreter puts before a compile error.
var sourcePosition = regexp.MustCompile(`(^|\s)(\d+):(\d+):`)

// bodyLines counts the lines of an interpreter message from the program body: the host's header takes the first line of
// the file, so the body's first line is the file's second.
func bodyLines(message string) string {
	return sourcePosition.ReplaceAllStringFunc(message, func(match string) string {
		parts := sourcePosition.FindStringSubmatch(match)
		line, _ := strconv.Atoi(parts[2])
		return fmt.Sprintf("%sline %d, column %s:", parts[1], max(line-1, 1), parts[3])
	})
}
