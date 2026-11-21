package chat

import "time"

// Step is what a chat is doing besides its reply, and when it began. The palette shows it with the elapsed time.
type Step struct {
	Label string
	Since time.Time
}

// SetStep shows what the chat is doing besides its reply, from now until ClearStep. It runs on the UI thread.
func (s *Service) SetStep(id, label string) {
	s.steps[id] = Step{Label: label, Since: s.opts.Clock.Now()}
}

// ClearStep ends the step the chat shows.
func (s *Service) ClearStep(id string) {
	delete(s.steps, id)
}

// StepView is a step as a view shows it: its label and how long it has run, on the service's clock.
type StepView struct {
	Label   string
	Elapsed time.Duration
}

// StepOf returns the step the chat is doing, if any, with how long it has run.
func (s *Service) StepOf(id string) (StepView, bool) {
	step, ok := s.steps[id]
	if !ok {
		return StepView{}, false
	}
	return StepView{Label: step.Label, Elapsed: s.opts.Clock.Now().Sub(step.Since)}, true
}
