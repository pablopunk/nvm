package diag

import (
	"strconv"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Reporter is the safego.Reporter the composition root installs. Each failure is recorded for the
// Diagnostics view and written to the log under the same reference, so a support request matches the file.
type Reporter struct {
	log *logx.Logger
	rec *Recorder
}

// NewReporter returns a reporter that logs to log and records into rec.
func NewReporter(log *logx.Logger, rec *Recorder) *Reporter {
	return &Reporter{log: log, rec: rec}
}

// Report implements safego.Reporter. It returns the reference it recorded.
func (r *Reporter) Report(f safego.Failure) string {
	attributes := map[string]string{"attempt": strconv.Itoa(f.Attempt)}
	if f.Panic {
		attributes["panic"] = "true"
	}
	if f.Disabled {
		attributes["disabled"] = "true"
	}
	ref := r.rec.Record(f.Name, f.Err, attributes)
	r.log.Error("component failed", logx.F{
		"ref":       ref,
		"component": f.Name,
		"panic":     f.Panic,
		"disabled":  f.Disabled,
		"attempt":   f.Attempt,
		"error":     f.Err,
		"frames":    stackFrames(f.Stack),
	})
	return ref
}
