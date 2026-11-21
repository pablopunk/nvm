package apps

// FirstSeen remembers when each app was first listed. Apps present before the baseline is confirmed are dated 0
// ("older"), so the first scan does not mark the whole machine as new. Initialized turns true once a complete
// baseline scan has run.
type FirstSeen struct {
	ByID        map[string]int64 `json:"byId"`
	Initialized bool             `json:"initialized"`
}

// TrackFirstSeen dates the apps against state and returns the dated list, the next state and whether the state
// changed. identityKey names an app by its path (normalized per OS); now is the moment a new app is first seen,
// used only once the baseline is initialized. initializeBaseline confirms the baseline on this scan.
func TrackFirstSeen(apps []App, state FirstSeen, initializeBaseline bool, now int64, identityKey func(string) string) ([]App, FirstSeen, bool) {
	next := FirstSeen{ByID: make(map[string]int64, len(state.ByID)), Initialized: state.Initialized}
	for id, at := range state.ByID {
		next.ByID[id] = at
	}
	changed := false
	tracked := make([]App, 0, len(apps))
	for _, app := range apps {
		id := identityKey(firstNonEmpty(app.Path, app.ID, app.Name))
		if _, known := next.ByID[id]; !known {
			at := int64(0)
			if state.Initialized {
				at = now
			}
			next.ByID[id] = at
			changed = true
		}
		app.FirstSeenAt = next.ByID[id]
		tracked = append(tracked, app)
	}
	nextInitialized := state.Initialized || initializeBaseline
	if nextInitialized != state.Initialized {
		changed = true
	}
	next.Initialized = nextInitialized
	return tracked, next, changed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
