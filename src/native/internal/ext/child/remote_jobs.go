package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteJobs reads and changes the background jobs through the host. A failed read reads as none.
type remoteJobs struct {
	link *hostLink
}

func (j remoteJobs) List() []api.Job {
	var jobs []api.Job
	_ = j.link.Call(protocol.ServiceJobs, protocol.OpList, nil, &jobs)
	return jobs
}

// Run asks the host to run a job now, and returns the reason it refused.
func (j remoteJobs) Run(id string) error {
	return j.link.Call(protocol.ServiceJobs, protocol.OpRun, protocol.JobArgs{ID: id}, nil)
}

// SetEnabled asks the host to turn a job on or off.
func (j remoteJobs) SetEnabled(id string, enabled bool) error {
	return j.link.Call(protocol.ServiceJobs, protocol.OpSetEnabled, protocol.JobEnabledArgs{ID: id, Enabled: enabled}, nil)
}

// ClearError asks the host to clear the last error of a job.
func (j remoteJobs) ClearError(id string) error {
	return j.link.Call(protocol.ServiceJobs, protocol.OpClearError, protocol.JobArgs{ID: id}, nil)
}
