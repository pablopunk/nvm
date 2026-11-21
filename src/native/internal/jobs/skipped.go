package jobs

// Skipped records a missing prerequisite without counting a failure, retrying or halting the job.
// The declaration and its ordinary event/interval schedule remain intact. Only pre-effect checks may return it.
type Skipped struct{ Reason string }

func (s Skipped) Error() string { return s.Reason }
