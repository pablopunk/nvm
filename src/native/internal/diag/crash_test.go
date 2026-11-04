package diag

import (
	"strings"
	"testing"
)

// TestPanicHandlerRecordsThenPanicsAgain drives the documented usage: the handler is deferred directly in
// the panicking function. The panic value must be re-raised unchanged, and the record must exist under the
// component main.
func TestPanicHandlerRecordsThenPanicsAgain(t *testing.T) {
	p := newPipeline(t)
	var rethrown any
	func() {
		defer func() { rethrown = recover() }()
		func() {
			defer InstallPanicHandler(p.reporter)()
			panic("fatal " + canary)
		}()
	}()

	if rethrown == nil {
		t.Fatal("the handler swallowed the panic")
	}
	if text, _ := rethrown.(string); text != "fatal "+canary {
		t.Fatalf("re-raised value = %v, want the original", rethrown)
	}
	recent := p.recorder.Recent()
	if len(recent) != 1 || recent[0].Component != "main" || recent[0].Attributes["panic"] != "true" {
		t.Fatalf("recent = %+v, want one main panic record", recent)
	}
	if strings.Contains(recent[0].Message, canary) {
		t.Fatalf("canary survived into the crash record: %q", recent[0].Message)
	}
}

func TestPanicHandlerDoesNothingWithoutAPanic(t *testing.T) {
	p := newPipeline(t)
	func() {
		defer InstallPanicHandler(p.reporter)()
	}()
	if len(p.recorder.Recent()) != 0 {
		t.Fatal("the handler recorded a failure that never happened")
	}
}
