package services

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// JobsSurface is the host's side of the background jobs: the jobs the scheduler holds, and the controls that run one,
// enable or disable it and clear its error. Its errors are the sentences an extension shows.
type JobsSurface interface {
	List() []api.Job
	Run(id string) error
	SetEnabled(id string, enabled bool) error
	ClearError(id string) error
}

// serveJobs answers the background jobs. Listing needs no declaration; every change needs the jobs declaration.
func (h *Handler) serveJobs(call protocol.ServiceCall) (any, error) {
	if h.deps.Jobs == nil {
		return nil, unavailable("background jobs are not available in this build")
	}
	if call.Op == protocol.OpList {
		return h.deps.Jobs.List(), nil
	}
	if err := h.require(api.CapabilityJobs); err != nil {
		return nil, err
	}
	switch call.Op {
	case protocol.OpRun:
		id, err := jobIDArg(call.Args)
		if err != nil {
			return nil, err
		}
		return nil, h.deps.Jobs.Run(id)
	case protocol.OpSetEnabled:
		return nil, h.jobSetEnabled(call.Args)
	case protocol.OpClearError:
		id, err := jobIDArg(call.Args)
		if err != nil {
			return nil, err
		}
		return nil, h.deps.Jobs.ClearError(id)
	}
	return nil, invalid("jobs has no operation %q", call.Op)
}

// jobSetEnabled turns a job on or off, as its arguments say.
func (h *Handler) jobSetEnabled(raw json.RawMessage) error {
	var args protocol.JobEnabledArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	if args.ID == "" {
		return invalid("a job needs an id")
	}
	return h.deps.Jobs.SetEnabled(args.ID, args.Enabled)
}

// jobIDArg decodes the id of the job an operation acts on, which must be named.
func jobIDArg(raw json.RawMessage) (string, error) {
	var args protocol.JobArgs
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if args.ID == "" {
		return "", invalid("a job needs an id")
	}
	return args.ID, nil
}
