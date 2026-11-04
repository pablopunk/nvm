package analytics

import (
	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// FailureReporter is the safego reporter the composition root installs. It reports each failure through next, which
// logs it and keeps its record, and then sends that record as an $exception event under the same support reference.
type FailureReporter struct {
	next    safego.Reporter
	client  *Client
	records *diag.Recorder
}

// NewFailureReporter chains a failure to next, then to the analytics client.
func NewFailureReporter(next safego.Reporter, client *Client, records *diag.Recorder) *FailureReporter {
	return &FailureReporter{next: next, client: client, records: records}
}

// Report implements safego.Reporter. It returns the reference next recorded.
func (r *FailureReporter) Report(f safego.Failure) string {
	ref := r.next.Report(f)
	if record, ok := r.records.Find(ref); ok {
		r.client.Exception(record)
	}
	return ref
}

// ReportProblem records a problem report that carries no text from the person or the app: a support reference and a
// time. The person copies the reference when they contact support, and the report is sent when diagnostics are on.
func ReportProblem(records *diag.Recorder, client *Client) string {
	ref := records.Record(ProblemComponent, nil, nil)
	client.SupportReport(ref)
	return ref
}
