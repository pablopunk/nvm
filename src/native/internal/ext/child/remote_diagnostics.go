package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteDiagnostics reads the support records from the host. A failed read reads as none, and a failed export as no text.
type remoteDiagnostics struct {
	link *hostLink
}

func (d remoteDiagnostics) Recent() []api.DiagnosticRecord {
	var records []api.DiagnosticRecord
	_ = d.link.Call(protocol.ServiceDiagnostics, protocol.OpRecent, nil, &records)
	return records
}

func (d remoteDiagnostics) Export(ref string) (string, error) {
	var text string
	err := d.link.Call(protocol.ServiceDiagnostics, protocol.OpExport, protocol.ExportArgs{Ref: ref}, &text)
	return text, err
}

// ReportProblem files a problem report and returns its reference. A failed exchange reads as no reference.
func (d remoteDiagnostics) ReportProblem() string {
	var ref string
	_ = d.link.Call(protocol.ServiceDiagnostics, protocol.OpReport, nil, &ref)
	return ref
}

// Reporting reads how records are reported now. A failed exchange reads as an empty state, which the page labels as stored.
func (d remoteDiagnostics) Reporting() string {
	var state string
	_ = d.link.Call(protocol.ServiceDiagnostics, protocol.OpReporting, nil, &state)
	return state
}
