package app

import (
	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// diagnosticsSurface serves the support records to the extension API: the recorder's records and their safe text, a
// problem report, and how records are reported now.
type diagnosticsSurface struct {
	p *App
}

// Recent lists the recorded errors, newest first, in the extension API's shape.
func (s diagnosticsSurface) Recent() []api.DiagnosticRecord {
	if s.p.recorder == nil {
		return nil
	}
	records := s.p.recorder.Recent()
	out := make([]api.DiagnosticRecord, 0, len(records))
	for _, record := range records {
		out = append(out, apiDiagnosticRecord(record))
	}
	return out
}

// Export is the safe text of one recorded error, and false when the recorder does not hold the reference.
func (s diagnosticsSurface) Export(ref string) (string, bool) {
	if s.p.recorder == nil {
		return "", false
	}
	text := s.p.recorder.Export(ref)
	return text, text != ""
}

// ReportProblem files a problem report and returns its reference, which is empty before the recorder opens.
func (s diagnosticsSurface) ReportProblem() string {
	if s.p.recorder == nil {
		return ""
	}
	return analytics.ReportProblem(s.p.recorder, s.p.analytics)
}

// Reporting is how records are reported now.
func (s diagnosticsSurface) Reporting() string {
	return apiReporting(s.p.reportingState())
}

// apiDiagnosticRecord converts a recorded error for the extension API. The fields are the recorder's own schema.
func apiDiagnosticRecord(record diag.Record) api.DiagnosticRecord {
	return api.DiagnosticRecord{
		Ref: record.Ref, Component: record.Component, Time: record.Time, Message: record.Message,
		Frames: record.Frames, Attributes: record.Attributes, Dropped: record.Dropped,
	}
}

// apiReporting names a reporting state for the extension API.
func apiReporting(state analytics.Reporting) string {
	switch state {
	case analytics.Off:
		return api.DiagnosticsOff
	case analytics.Requested:
		return api.DiagnosticsRequested
	}
	return api.DiagnosticsStored
}
