package diag

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/safego"
)

// TestPanicReachesLogAndRecorderUnderOneReference runs a panic through safego with the diag reporter. The
// reference returned to the caller must name the record and the log line, and the panic value carries a
// canary that must reach neither.
func TestPanicReachesLogAndRecorderUnderOneReference(t *testing.T) {
	p := newPipeline(t)
	err := safego.Run("provider.search", func(context.Context) error {
		panic("token=" + canary)
	}, safego.WithReporter(p.reporter))

	var panicked *safego.PanicError
	if !errors.As(err, &panicked) {
		t.Fatalf("Run = %v, want *PanicError", err)
	}
	record, ok := p.recorder.Find(panicked.Ref)
	if !ok {
		t.Fatalf("no record under the reference %q", panicked.Ref)
	}
	if record.Component != "provider.search" || len(record.Frames) == 0 {
		t.Fatalf("record = %+v, want the component and stack frames", record)
	}
	if record.Attributes["panic"] != "true" || record.Attributes["attempt"] != "1" {
		t.Fatalf("attributes = %v", record.Attributes)
	}

	if err := p.logger.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	file, err := p.files.ReadFile("/logs/nvm.log")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(file), panicked.Ref) {
		t.Fatalf("the log does not carry the reference %s:\n%s", panicked.Ref, file)
	}
	for _, surface := range []string{string(file), record.Message, recorderExport(p, panicked.Ref)} {
		if strings.Contains(surface, canary) || strings.Contains(surface, canary[:12]) {
			t.Fatalf("canary reached an output:\n%s", surface)
		}
	}
}

func TestReporterRecordsDisabledCallsAsDisabled(t *testing.T) {
	p := newPipeline(t)
	ref := p.reporter.Report(safego.Failure{Name: "offender", Panic: true, Err: errors.New("boom"), Attempt: 3, Disabled: true})
	record, ok := p.recorder.Find(ref)
	if !ok || record.Attributes["disabled"] != "true" || record.Attributes["attempt"] != "3" {
		t.Fatalf("record = %+v, want disabled attempt 3", record)
	}
}

func TestSecretFormsNeverReachTheLogFile(t *testing.T) {
	p := newPipeline(t)
	for _, err := range secretForms() {
		p.reporter.Report(safego.Failure{Name: "provider", Err: err, Attempt: 1})
	}
	if err := p.logger.Flush(); err != nil {
		t.Fatal(err)
	}
	file, err := p.files.ReadFile("/logs/nvm.log")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(file), canary) {
		t.Fatalf("canary reached the log file:\n%s", file)
	}
}

func recorderExport(p pipeline, ref string) string {
	return p.recorder.Export(ref)
}
