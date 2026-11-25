package codemode

// keepPlan keeps what a rule's prepare step learned from the host, such as a prepared shortcut change or the job a
// call names, for the call being asked about or run. The rule's describe and journal read it back with keptPlan, so
// the question and the journal line name host data, not the program's words. Each call starts with no plan.
func keepPlan[T any](g *gate, plan T) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.plan = plan
}

// keptPlan returns the plan the current call's prepare step kept, or the zero T when it kept none of that type.
func keptPlan[T any](g *gate) T {
	g.mu.Lock()
	defer g.mu.Unlock()
	plan, _ := g.plan.(T)
	return plan
}

func (g *gate) forgetPlan() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.plan = nil
}
