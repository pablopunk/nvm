package api

import "time"

// DiagnosticsProblemComponent is the component of a record the person filed with Report a Problem. Its record has no
// error text, only a reference the person can copy.
const DiagnosticsProblemComponent = "support.report"

// Reporting states of a record, as the Recent Errors page labels them. Stored means the record stays on this computer
// (a build that never sends). Off means the person turned diagnostics off. Requested means the record may be sent, and
// its arrival at the endpoint is not confirmed.
const (
	DiagnosticsStored    = "stored"
	DiagnosticsOff       = "off"
	DiagnosticsRequested = "requested"
)

// DiagnosticRecord is one error the host recorded. Its fields are the whole schema: a support reference, a component
// label the code defines, the time, redacted error text, a few stack frames (function and file base name only),
// allow-listed attributes and a count of attributes the host refused. It never carries prompts, clipboard or screen
// content, file contents or paths.
type DiagnosticRecord struct {
	Ref        string            `json:"ref"`
	Component  string            `json:"component"`
	Time       time.Time         `json:"time"`
	Message    string            `json:"message,omitempty"`
	Frames     []string          `json:"frames,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Dropped    int               `json:"dropped,omitempty"`
}

// Diagnostics reads the records the host keeps for support, files a problem report and says how records are reported.
// Its reads need no declaration: a record is the extension's own support reference, and the export has no more than
// the record does.
type Diagnostics interface {
	// Recent returns the records, newest first.
	Recent() []DiagnosticRecord
	// Export returns the safe plain text of one record, the text a support request copies. Its error names a reference
	// the host does not hold.
	Export(ref string) (string, error)
	// ReportProblem records a problem report and returns its reference. The report has no screen content.
	ReportProblem() string
	// Reporting says how records are reported now: DiagnosticsStored, DiagnosticsOff or DiagnosticsRequested.
	Reporting() string
}
