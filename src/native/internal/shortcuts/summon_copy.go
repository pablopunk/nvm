package shortcuts

// SummonCopy is the banner for a summon shortcut that does not work: what is wrong, how to get around it, and
// the label of the one button that fixes it. Alert marks a shortcut that is set but unavailable.
type SummonCopy struct {
	Title       string
	Detail      string
	ActionLabel string
	Alert       bool
}

// SummonNotice returns the banner for st, or false when there is nothing to show: the shortcut is bound, or it
// is still being retried. label spells an accelerator for the user.
func SummonNotice(st Status, label func(accelerator string) string) (SummonCopy, bool) {
	if st.Registered || st.Retrying {
		return SummonCopy{}, false
	}
	if st.Accelerator == "" {
		return SummonCopy{Title: "No shortcut set", Detail: "Set one to open Nevermind from anywhere.", ActionLabel: "Set shortcut"}, true
	}
	return SummonCopy{
		Title:       label(st.Accelerator) + " is unavailable",
		Detail:      unavailableDetail(st.Recovery, label),
		ActionLabel: "Change shortcut",
		Alert:       true,
	}, true
}

// unavailableDetail names the recovery shortcut when one is bound, and the app launcher when none is.
func unavailableDetail(recovery string, label func(accelerator string) string) string {
	if recovery == "" {
		return "Reopen Nevermind from your app launcher."
	}
	return "Use " + label(recovery) + " to reopen Nevermind."
}
