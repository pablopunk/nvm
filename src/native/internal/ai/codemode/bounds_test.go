package codemode

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

func withProgramBudget(hard time.Duration) fixtureOption {
	return func(o *Options) {
		o.Supervisor.Budgets = map[string]supervisor.Budget{protocol.MethodProgram: {Hard: hard}}
	}
}

func TestProgramThatNeverEndsIsStoppedAtItsBudgetAndReaped(t *testing.T) {
	f := newFixture(t, withProgramBudget(400*time.Millisecond))
	started := time.Now()
	result := f.run("for {\n}\nreturn nil, nil")
	if !result.Failed || !strings.Contains(result.Text, "The program was stopped: it computed for more than its limit") {
		t.Fatalf("result = %+v", result)
	}
	if took := time.Since(started); took > 10*time.Second {
		t.Fatalf("the run took %s", took)
	}
	noChildren(t)
	f.noStagedPrograms()
}

// The user's time answering a question is not the program's computing time.
func TestWaitingForTheUserDoesNotUseTheComputeBudget(t *testing.T) {
	f := newFixture(t, withProgramBudget(time.Second))
	f.note("# Old ideas")
	f.steps.decide = func(*fakeRow, context.Context) (bool, error) {
		time.Sleep(1500 * time.Millisecond)
		return true, nil
	}
	result := f.run(deleteOld)
	if result.Failed || !strings.Contains(result.Text, `Result: "deleted"`) {
		t.Fatalf("a slow answer ended the program: %+v", result)
	}
}

func TestResultOverTheBoundIsAReadableFailure(t *testing.T) {
	f := newFixture(t)
	result := f.run("return strings.Repeat(\"x\", 100000), nil")
	if !result.Failed {
		t.Fatalf("an oversize result was accepted: %.80s", result.Text)
	}
	contains(t, result.Text, "return less than 65536")
}

func TestResultJustUnderTheBoundComesBack(t *testing.T) {
	f := newFixture(t)
	result := f.run("return strings.Repeat(\"x\", 60000), nil")
	if result.Failed || len(result.Text) < 60000 {
		t.Fatalf("result = failed %v, %d bytes", result.Failed, len(result.Text))
	}
}

func TestSourceOverTheLimitIsRefusedBeforeAnythingIsStaged(t *testing.T) {
	f := newFixture(t)
	result := f.run(strings.Repeat("x := 1\n_ = x\n", 2000) + "return nil, nil")
	if !result.Failed {
		t.Fatal("an oversize program ran")
	}
	contains(t, result.Text, "The program was not run", "keep it under 8192")
	f.noStagedPrograms()
	if rows := f.steps.snapshot(); len(rows) != 0 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestRefusedProgramsNeverStartAProcess(t *testing.T) {
	f := newFixture(t)
	for _, body := range []string{"data, _ := os.ReadFile(\"/etc/hostname\")\nreturn string(data), nil", "go func() {}()\nreturn nil, nil", "import \"os\"\nreturn nil, nil"} {
		result := f.run(body)
		if !result.Failed || !strings.HasPrefix(result.Text, "The program was not run: ") {
			t.Fatalf("%q: result = %+v", body, result)
		}
	}
	if childProcesses(t) != 0 {
		t.Fatal("a refused program started a child")
	}
	f.noStagedPrograms()
}

func TestCompileErrorsComeBackWithTheBodysLineNumbers(t *testing.T) {
	f := newFixture(t)
	result := f.run("x := 1\n_ = x\nreturn missing(), nil")
	if !result.Failed {
		t.Fatal("a program that does not compile ran")
	}
	contains(t, result.Text, "The program did not compile", "line 3", "Nothing ran.")
	f.noStagedPrograms()
	noChildren(t)
}

func TestInterpreterPanicsBecomeProgramErrors(t *testing.T) {
	f := newFixture(t)
	for _, body := range []string{"var m map[string]int\nm[\"a\"] = 1\nreturn nil, nil", "values := []int{1}\nreturn values[5], nil"} {
		result := f.run(body)
		if !result.Failed || !strings.Contains(result.Text, "The program returned an error: extension panicked") {
			t.Fatalf("%q: result = %+v", body, result)
		}
	}
	// The next run works: a panic costs one child, never the host.
	if result := f.run("return 1, nil"); result.Failed {
		t.Fatalf("a later run failed: %+v", result)
	}
	noChildren(t)
}

// A crash that takes the child's process down, not just the program, is a clean failure and the host carries on.
func TestChildCrashIsACleanFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the address-space limit that makes this allocation fatal exists on Linux only")
	}
	f := newFixture(t)
	result := f.run("block := make([]byte, 6<<30)\nreturn len(block), nil")
	if !result.Failed || !strings.Contains(result.Text, "The program stopped unexpectedly") {
		t.Fatalf("result = %+v", result)
	}
	if result := f.run("return 1, nil"); result.Failed {
		t.Fatalf("the run after a crash failed: %+v", result)
	}
	noChildren(t)
	f.noStagedPrograms()
}

// Memory that stays under the address-space limit is caught by the resident watch.
func TestResidentMemoryWatchStopsAProgramThatHoldsTooMuch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process sizes are read on Linux only")
	}
	f := newFixture(t, func(o *Options) { o.Supervisor.ResidentLimit = 120 << 20 })
	result := f.run(`block := make([]byte, 250<<20)
for i := 0; i < len(block); i += 4096 { block[i] = 1 }
time.Sleep(15 * time.Second)
return len(block), nil`)
	if !result.Failed {
		t.Fatalf("a program holding 250 MB ran to the end: %+v", result)
	}
	contains(t, result.Text, "memory limit exceeded")
	noChildren(t)
}
