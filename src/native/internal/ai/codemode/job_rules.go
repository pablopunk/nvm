package codemode

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// jobPlan is the job a call names, as the host lists it, and for setEnabled the state the call asks for.
type jobPlan struct {
	job     api.Job
	enabled bool
}

// jobRules are the operations of the background jobs a program may use. Turning a job on or off changes what
// Nevermind does on its own, and running one runs extension code, so both ask every time; clearing an error does not.
func jobRules() map[key]rule {
	op := func(name string) key { return key{protocol.ServiceJobs, name} }
	return map[key]rule{
		op(protocol.OpList): {class: ClassRead, label: "Listing background jobs", describe: describeNothing, journal: journalNone},
		op(protocol.OpSetEnabled): {class: ClassSetting, label: "Changing a background job", prepare: prepareJobSetEnabled,
			describe: describeJobSetEnabled, journal: journalJobSetEnabled},
		op(protocol.OpRun): {class: ClassExternal, label: "Running a background job", prepare: prepareJob,
			describe: describeJobRun, journal: journalJobRun},
		op(protocol.OpClearError): {class: ClassWrite, label: "Clearing a job error", prepare: prepareJob,
			describe: describeJobName, journal: journalJobErrorCleared},
	}
}

var errJobsUnavailable = errors.New("background jobs are not available in this build")

// prepareJob looks the job up in the host's list, so the question names it by title. An unknown id fails without asking.
func prepareJob(g *gate, call protocol.ServiceCall) (protocol.ServiceCall, error) {
	var args protocol.JobArgs
	_ = json.Unmarshal(call.Args, &args)
	_, err := g.planJob(args.ID, false)
	return call, err
}

// prepareJobSetEnabled looks the job up and refuses, without asking, a change to the state it already has.
func prepareJobSetEnabled(g *gate, call protocol.ServiceCall) (protocol.ServiceCall, error) {
	var args protocol.JobEnabledArgs
	_ = json.Unmarshal(call.Args, &args)
	plan, err := g.planJob(args.ID, args.Enabled)
	if err != nil {
		return call, err
	}
	if plan.job.Enabled == args.Enabled {
		return call, fmt.Errorf("“%s” is already %s; nothing changed", plan.job.Title, onOrOff(args.Enabled))
	}
	return call, nil
}

func (g *gate) planJob(id string, enabled bool) (jobPlan, error) {
	plan := jobPlan{job: api.Job{ID: id}, enabled: enabled}
	defer func() { keepPlan(g, plan) }()
	if g.jobs == nil {
		return plan, errJobsUnavailable
	}
	for _, job := range g.jobs.List() {
		if job.ID == id {
			plan.job = job
			return plan, nil
		}
	}
	return plan, fmt.Errorf("no background job has the id %q; list them first", id)
}

// jobName is how a row names the job: its title from the host, or its id when the host has none.
func jobName(job api.Job) string {
	if job.Title == "" {
		return clip(job.ID, 40)
	}
	return "“" + job.Title + "”"
}

func describeJobName(g *gate, _ protocol.ServiceCall) string {
	return "Background job " + jobName(keptPlan[jobPlan](g).job)
}

func describeJobSetEnabled(g *gate, _ protocol.ServiceCall) string {
	plan := keptPlan[jobPlan](g)
	text := fmt.Sprintf("Turn %s the background job %s", onOrOff(plan.enabled), jobName(plan.job))
	if plan.job.Owner == api.JobOwnerExtension && plan.job.Scope != "" {
		text += " of " + plan.job.Scope
	}
	return text
}

func describeJobRun(g *gate, _ protocol.ServiceCall) string {
	return fmt.Sprintf("Run the background job %s now", jobName(keptPlan[jobPlan](g).job))
}

func journalJobSetEnabled(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	plan := keptPlan[jobPlan](g)
	return fmt.Sprintf("turned %s the background job %s (the user allowed it)", onOrOff(plan.enabled), jobName(plan.job))
}

func journalJobRun(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	return fmt.Sprintf("started the background job %s (the user allowed it)", jobName(keptPlan[jobPlan](g).job))
}

func journalJobErrorCleared(g *gate, _ protocol.ServiceCall, _ protocol.ServiceReply) string {
	return fmt.Sprintf("cleared the last error of the background job %s", jobName(keptPlan[jobPlan](g).job))
}

func onOrOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}
