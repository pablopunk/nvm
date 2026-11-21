package services

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// DiagnosticsSurface is the host's side of the support records: the recent ones, the safe text of one, a problem report
// and the reporting state the records carry.
type DiagnosticsSurface interface {
	Recent() []api.DiagnosticRecord
	Export(ref string) (string, bool)
	ReportProblem() string
	Reporting() string
}

// serveDiagnostics answers the support records. Without a surface the host keeps none, so the reads are empty and a
// report is unavailable.
func (h *Handler) serveDiagnostics(call protocol.ServiceCall) (any, error) {
	if h.deps.Diagnostics == nil {
		return nil, unavailable("diagnostics are not available in this build")
	}
	switch call.Op {
	case protocol.OpRecent:
		return h.deps.Diagnostics.Recent(), nil
	case protocol.OpExport:
		return h.diagnosticsExport(call.Args)
	case protocol.OpReport:
		return h.deps.Diagnostics.ReportProblem(), nil
	case protocol.OpReporting:
		return h.deps.Diagnostics.Reporting(), nil
	}
	return nil, invalid("diagnostics has no operation %q", call.Op)
}

// diagnosticsExport reads the safe text of one record. A reference the host does not hold is an error the extension sees.
func (h *Handler) diagnosticsExport(raw json.RawMessage) (string, error) {
	var args protocol.ExportArgs
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if args.Ref == "" {
		return "", invalid("an export needs a support reference")
	}
	text, found := h.deps.Diagnostics.Export(args.Ref)
	if !found {
		return "", invalid("there is no recorded error %q", args.Ref)
	}
	return text, nil
}
