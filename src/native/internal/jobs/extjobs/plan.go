package extjobs

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/jobs"
)

// plan is the job one command gets, with the folder watches its triggers need.
type plan struct {
	extensionID string
	command     api.Command
	def         jobs.Definition
	watches     []fileWatch
}

// planFor returns the job for a command, or false when the command should not be a job. A command runs as a job when
// it declares a trigger the host runs, or when it asks for a background or no-view mode (then it runs on demand from
// the Background Tasks list). A view command without triggers stays a palette command.
func planFor(extensionID string, command api.Command, home string) (plan, bool) {
	id := jobIDFor(extensionID, command.ID)
	triggers, watches := mapTriggers(id, command.Triggers, home)
	background := command.Mode == api.ModeBackground || command.Mode == api.ModeNoView
	if len(triggers) == 0 && !background {
		return plan{}, false
	}
	return plan{
		extensionID: extensionID,
		command:     command,
		def: jobs.Definition{
			ID:       id,
			Title:    command.Title,
			Owner:    jobs.OwnerExtension,
			Scope:    extensionID,
			Triggers: triggers,
		},
		watches: watches,
	}, true
}
