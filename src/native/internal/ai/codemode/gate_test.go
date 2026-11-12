package codemode

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// The class of every operation a program may use is decided here, in Go. This table is the review surface: a new
// operation, or a changed class, shows up as a diff of this test.
func TestNotesOperationsAreClassifiedByEffect(t *testing.T) {
	want := map[string]Class{
		protocol.OpList: ClassRead, protocol.OpGet: ClassRead, protocol.OpSearch: ClassRead,
		protocol.OpRecent: ClassWrite, protocol.OpCreate: ClassWrite, protocol.OpCreateWith: ClassWrite, protocol.OpUpdate: ClassWrite,
		protocol.OpDelete: ClassDestructive,
	}
	rules := notesRules()
	if len(rules) != len(want) {
		t.Fatalf("the notes table has %d rules; this test pins %d", len(rules), len(want))
	}
	for op, class := range want {
		rule, ok := rules[key{protocol.ServiceNotes, op}]
		if !ok || rule.class != class {
			t.Errorf("notes.%s = %+v (listed %v); want %s", op, rule.class, ok, class)
		}
		if rule.label == "" || rule.describe == nil || rule.journal == nil {
			t.Errorf("notes.%s lacks a label, a description or a journal line", op)
		}
	}
}

func TestOnlyDestructiveAndExternalAskEachTimeAndSensitiveReadsAskOnce(t *testing.T) {
	cases := map[Class][3]bool{ // asksEachTime, asksOncePerService, changesData
		ClassRead: {false, false, false}, ClassWrite: {false, false, true}, ClassSensitiveRead: {false, true, false},
		ClassDestructive: {true, false, true}, ClassExternal: {true, false, true},
	}
	for class, want := range cases {
		if got := [3]bool{class.asksEachTime(), class.asksOncePerService(), class.changesData()}; got != want {
			t.Errorf("%s = %v; want %v", class, got, want)
		}
	}
}

// Every service the notes slice does not use is refused, including the ones the extension services leave open to any
// extension. The refusal reaches the program as a permission error and the journal as a refusal.
func TestProgramCannotReachAnythingBesideNotes(t *testing.T) {
	f := newFixture(t)
	result := f.run(`var refused []string
if _, _, err := ctx.Secrets().Get("token"); err != nil { refused = append(refused, "secrets") }
var stored string
if _, err := ctx.Storage().Get("k", &stored); err != nil { refused = append(refused, "storage") }
if _, err := ctx.Clipboard().List(api.ClipboardQuery{}); err != nil { refused = append(refused, "clipboard") }
if _, err := ctx.Network().Fetch(api.FetchRequest{URL: "https://example.com"}); err != nil { refused = append(refused, "network") }
if _, err := ctx.Desktop().Shell().Exec("ls", nil, api.ShellOptions{}); err != nil { refused = append(refused, "shell") }
return refused, nil`)
	if result.Failed {
		t.Fatalf("failed: %s", result.Text)
	}
	contains(t, result.Text, `["secrets","storage","clipboard","network","shell"]`)
	blocked := 0
	for _, row := range f.steps.snapshot() {
		if row.label == "Blocked a call" && row.status == "refused" {
			blocked++
		}
	}
	if blocked != 5 {
		t.Fatalf("%d blocked rows; want 5: %+v", blocked, f.steps.snapshot())
	}
	contains(t, result.Text, "refused secrets.get", "refused network.fetch")
}

func TestProgramCallsPastTheLimitAreRefused(t *testing.T) {
	f := newFixture(t, func(o *Options) { o.MaxServiceCalls = 5 })
	result := f.run(`count := 0
var last error
for i := 0; i < 20; i++ {
	if _, err := ctx.Notes().List(); err != nil { last = err; break }
	count++
}
return count, last`)
	contains(t, result.Text, "a program may make at most 5 Nevermind calls")
	if result.Failed != true {
		t.Fatal("the error the program returned must end the run failed")
	}
}

// The gate answers only the services it was given; a gate with no handler behind it never reaches one for a call it
// refuses.
func TestGateRefusesUnlistedCallsWithoutTouchingTheServices(t *testing.T) {
	g := newGate(gateConfig{rules: notesRules(), runCtx: context.Background()})
	reply := g.Serve(context.Background(), protocol.ServiceCall{Service: protocol.ServiceSecrets, Op: protocol.OpGet})
	if reply.Failure == nil || reply.Failure.Code != "permission" {
		t.Fatalf("reply = %+v; want a permission refusal", reply)
	}
	if entries := g.journal(); len(entries) != 1 || entries[0].outcome != outcomeRefused {
		t.Fatalf("journal = %+v", entries)
	}
}
