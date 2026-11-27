package codemode

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// answerFixture is a gate over a fake service with a read, a sensitive read and a command.
func answerFixture() (*gate, *fakeSteps, *stubServices) {
	hostWords := func(*gate, protocol.ServiceCall) string { return "the host's words" }
	rules := map[key]rule{
		{"fake", "look"}: {class: ClassRead, label: "Looking", describe: hostWords, journal: journalNone},
		{"fake", "peek"}: {class: ClassSensitiveRead, label: "Reading private data", describe: hostWords, journal: journalNone},
		{"fake", "run"}:  {class: ClassExternal, label: "Running a command", describe: hostWords, journal: journalNone},
	}
	steps := &fakeSteps{}
	inner := &stubServices{}
	g := newGate(gateConfig{rules: rules, runCtx: context.Background(), steps: steps})
	g.inner = inner
	return g, steps, inner
}

func serveOn(g *gate) func(op, args string) protocol.ServiceReply {
	return func(op, args string) protocol.ServiceReply {
		return g.Serve(context.Background(), protocol.ServiceCall{Service: "fake", Op: op, Args: json.RawMessage(args)})
	}
}

func asked(steps *fakeSteps) int {
	n := 0
	for _, row := range steps.snapshot() {
		if row.asked {
			n++
		}
	}
	return n
}

// A decline covers the exact operation and resource for the rest of the run: the same call fails at once with a clear
// reason and is never put to the user again, whatever its class.
func TestARetryAfterARefusalFailsWithoutAskingAgain(t *testing.T) {
	for _, op := range []string{"peek", "run"} {
		t.Run(op, func(t *testing.T) {
			g, steps, inner := answerFixture()
			serve := serveOn(g)
			steps.decide = decline
			serve(op, `{"id":"a"}`)
			steps.decide = allow
			reply := serve(op, `{"id":"a"}`)
			if reply.Failure == nil || reply.Failure.Code != api.CodePermission || !strings.Contains(reply.Failure.Message, "already declined this exact action") {
				t.Fatalf("the retry got %+v; want a permission failure that names the earlier decline", reply.Failure)
			}
			if asked(steps) != 1 || inner.count() != 0 {
				t.Fatalf("asked %d, ran %d; want the decline asked once and nothing run", asked(steps), inner.count())
			}
			if !strings.Contains(journalText(g.journal()), "declined it earlier in this chat") {
				t.Fatalf("journal = %s", journalText(g.journal()))
			}
		})
	}
}

// A decline covers only its own resource and operation: another resource, or another operation, is asked.
func TestARefusalDoesNotCoverAnotherResourceOrOperation(t *testing.T) {
	g, steps, inner := answerFixture()
	serve := serveOn(g)
	steps.decide = decline
	serve("peek", `{"id":"a"}`)
	steps.decide = allow
	if reply := serve("peek", `{"id":"b"}`); reply.Failure != nil {
		t.Fatalf("another resource failed: %+v", reply.Failure)
	}
	if reply := serve("run", `{"id":"a"}`); reply.Failure != nil {
		t.Fatalf("another operation failed: %+v", reply.Failure)
	}
	if asked(steps) != 3 || inner.count() != 2 {
		t.Fatalf("asked %d, ran %d; want each new question asked", asked(steps), inner.count())
	}
}
