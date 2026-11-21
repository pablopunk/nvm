package services

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakeDiagnostics is the host's support records: one record, its safe text, and the reporting state.
type fakeDiagnostics struct {
	reported int
}

func (f *fakeDiagnostics) Recent() []api.DiagnosticRecord {
	return []api.DiagnosticRecord{{Ref: "NVM-K7P2Q", Component: "calc.rates", Time: time.Date(2026, 11, 3, 9, 41, 7, 0, time.UTC)}}
}

func (f *fakeDiagnostics) Export(ref string) (string, bool) {
	if ref != "NVM-K7P2Q" {
		return "", false
	}
	return "Reference: NVM-K7P2Q\n", true
}

func (f *fakeDiagnostics) ReportProblem() string {
	f.reported++
	return "NVM-WXR4T"
}

func (f *fakeDiagnostics) Reporting() string { return api.DiagnosticsRequested }

func diagnosticsHandler(t *testing.T, surface DiagnosticsSurface) *Handler {
	t.Helper()
	_, deps := newDesktopHost(t)
	deps.Diagnostics = surface
	return desktopHandler(t, deps)
}

func TestRecentRecordsAreReadWithoutADeclaration(t *testing.T) {
	h := diagnosticsHandler(t, &fakeDiagnostics{})
	var got []api.DiagnosticRecord
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDiagnostics, protocol.OpRecent, nil)), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Ref != "NVM-K7P2Q" || got[0].Component != "calc.rates" {
		t.Fatalf("records = %+v", got)
	}
}

func TestExportReturnsTheSafeTextOfAKnownReference(t *testing.T) {
	h := diagnosticsHandler(t, &fakeDiagnostics{})
	var text string
	args := protocol.ExportArgs{Ref: "NVM-K7P2Q"}
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDiagnostics, protocol.OpExport, args)), &text); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if text != "Reference: NVM-K7P2Q\n" {
		t.Fatalf("text = %q", text)
	}
}

func TestExportRefusesAnUnknownOrEmptyReference(t *testing.T) {
	h := diagnosticsHandler(t, &fakeDiagnostics{})
	requireCode(t, serve(t, h, protocol.ServiceDiagnostics, protocol.OpExport, protocol.ExportArgs{Ref: "NVM-NOPE1"}), api.CodeInvalid, "NVM-NOPE1")
	requireCode(t, serve(t, h, protocol.ServiceDiagnostics, protocol.OpExport, protocol.ExportArgs{}), api.CodeInvalid, "reference")
}

func TestReportFilesOneReferenceAndReportingIsRead(t *testing.T) {
	surface := &fakeDiagnostics{}
	h := diagnosticsHandler(t, surface)
	var ref string
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDiagnostics, protocol.OpReport, nil)), &ref); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ref != "NVM-WXR4T" || surface.reported != 1 {
		t.Fatalf("ref = %q, reported = %d", ref, surface.reported)
	}
	var state string
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDiagnostics, protocol.OpReporting, nil)), &state); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if state != api.DiagnosticsRequested {
		t.Fatalf("reporting = %q", state)
	}
}

func TestDiagnosticsWithoutASurfaceIsUnavailable(t *testing.T) {
	h := diagnosticsHandler(t, nil)
	requireCode(t, serve(t, h, protocol.ServiceDiagnostics, protocol.OpRecent, nil), api.CodeUnavailable, "diagnostics")
}

func TestDiagnosticsRefusesAnUnlistedOperation(t *testing.T) {
	h := diagnosticsHandler(t, &fakeDiagnostics{})
	requireCode(t, serve(t, h, protocol.ServiceDiagnostics, "clear", nil), api.CodeInvalid, "diagnostics")
}
