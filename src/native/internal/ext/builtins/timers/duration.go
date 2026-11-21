package timers

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// durationQuery is what the root accepts after "timer" or "countdown": a number and an optional unit. A bare number is
// minutes.
var durationQuery = regexp.MustCompile(`^(?:timer\s+|countdown\s+)?(\d+(?:\.\d+)?)\s*(s|sec|m|min|minutes?|h|hr|hours?)?$`)

// parseTimer reads a query such as "timer 5m" or "countdown 90s" as a duration. A query without the word first is not a
// timer.
func parseTimer(query string) (time.Duration, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if !strings.HasPrefix(q, "timer") && !strings.HasPrefix(q, "countdown") {
		return 0, false
	}
	m := durationQuery.FindStringSubmatch(q)
	if m == nil {
		return 0, false
	}
	value, _ := strconv.ParseFloat(m[1], 64)
	switch {
	case strings.HasPrefix(m[2], "s"):
		return time.Duration(value * float64(time.Second)), true
	case strings.HasPrefix(m[2], "h"):
		return time.Duration(value * float64(time.Hour)), true
	}
	return time.Duration(value * float64(time.Minute)), true
}
