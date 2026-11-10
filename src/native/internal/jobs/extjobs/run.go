package extjobs

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/jobs"
)

var errExtensionsLoading = errors.New("the extensions are still loading")

// failureError carries an extension's own failure message into the job's history.
type failureError struct {
	failure *api.Failure
}

func (e failureError) Error() string {
	return e.failure.Message
}

// runCommand is the job's function: it runs the command through the extension host with a launch context built from
// the run. A failure the extension reports fails the run, so the scheduler backs off and halts a broken command. A
// view the command returns is not shown, since a background run has no palette.
func (a *Adapter) runCommand(p plan) func(context.Context, jobs.Run) error {
	return func(ctx context.Context, run jobs.Run) error {
		host := a.cfg.Host()
		if host == nil {
			return jobs.Skipped{Reason: errExtensionsLoading.Error()}
		}
		if support, ok := host.(interface{ OperationsSupported([]api.Operation) bool }); ok && !support.OperationsSupported(p.command.Requires) {
			return jobs.Skipped{Reason: "required operation is unavailable on this system"}
		}
		result, failure, err := host.RunCommand(ctx, p.extensionID, launchInvocation(p.command, run))
		if err != nil {
			return err
		}
		if failure != nil {
			return failureError{failure: failure}
		}
		if result.View != nil {
			a.logInfo("extension.background.viewIgnored", p.def.ID)
		}
		return nil
	}
}

// launchInvocation is the request that runs a command: the command's id and a launch context that says why it runs.
func launchInvocation(command api.Command, run jobs.Run) api.Invocation {
	event, _ := run.Payload["event"].(string)
	if event == "" {
		event = strings.TrimPrefix(run.Reason, "event:")
	}
	paths, _ := run.Payload[jobs.ChangedPaths].([]string)
	return api.Invocation{
		Ref: api.ActionRef{ID: command.ID},
		Launch: api.LaunchContext{
			Trigger:      triggerFor(command.Triggers, run),
			Event:        eventOf(run.Reason, event),
			Files:        fileRefs(paths),
			ChangedPaths: paths,
			Reason:       publicReason(run.Reason, event),
			StartedAtMs:  run.StartedAt.UnixMilli(),
		},
	}
}

// publicReason maps a scheduler reason onto the reasons the API documents: manual, startup, interval, event:<name>.
func publicReason(reason, event string) string {
	switch {
	case strings.HasPrefix(reason, "event:"):
		return "event:" + event
	case reason == jobs.ReasonMissed || reason == jobs.ReasonRetry:
		return jobs.ReasonInterval
	default:
		return reason
	}
}

// eventOf names the host event that started a run, or "" when none did.
func eventOf(reason, event string) string {
	if strings.HasPrefix(reason, "event:") {
		return event
	}
	return ""
}

// triggerFor returns the declared trigger that started the run. A folder event carries its own trigger; other runs
// match the declared trigger of the same kind.
func triggerFor(declared []api.Trigger, run jobs.Run) *api.Trigger {
	if trigger, ok := run.Payload["trigger"].(api.Trigger); ok {
		return &trigger
	}
	kind := triggerKindOf(run.Reason)
	for _, trigger := range declared {
		if trigger.Type == kind {
			return &trigger
		}
	}
	return nil
}

func triggerKindOf(reason string) api.TriggerType {
	switch {
	case reason == jobs.ReasonStartup:
		return api.TriggerStartup
	case reason == jobs.ReasonInterval, reason == jobs.ReasonMissed, reason == jobs.ReasonRetry:
		return api.TriggerInterval
	case strings.HasPrefix(reason, "event:"):
		return api.TriggerType(strings.TrimPrefix(reason, "event:"))
	default:
		return ""
	}
}

// fileRefs describes the changed paths as files for the command, from their names alone: a run never reads the disk.
func fileRefs(paths []string) []api.FileRef {
	refs := make([]api.FileRef, 0, len(paths))
	for _, path := range paths {
		name := filepath.Base(path)
		refs = append(refs, api.FileRef{
			Path:      path,
			Name:      name,
			Extension: files.ExtensionOf(name),
			Kind:      kindName(files.KindOf(name)),
		})
	}
	return refs
}

func kindName(kind files.Kind) string {
	switch kind {
	case files.KindImage:
		return "image"
	case files.KindVideo:
		return "video"
	default:
		return "file"
	}
}
