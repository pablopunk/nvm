package calc

import (
	"regexp"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // embedded zone database so named zones resolve on every platform
)

// currentTimePattern matches "time in Tokyo".
var currentTimePattern = regexp.MustCompile(`(?i)^time\s+in\s+(.+)$`)

// conversionTimePattern matches "5pm ldn in sf": a clock time in one zone asked for in another.
var conversionTimePattern = regexp.MustCompile(`(?i)^(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s+(.+?)\s+in\s+(.+)$`)

// zoneAliases maps normalized city and abbreviation names to IANA zones.
var zoneAliases = buildZoneAliases()

// buildZoneAliases declares the zone names the calculator knows, in the TypeScript original's order.
func buildZoneAliases() map[string]string {
	aliases := map[string]string{}
	add := func(zone string, names ...string) {
		aliases[normalizeZoneAlias(zone)] = zone
		for _, name := range names {
			aliases[normalizeZoneAlias(name)] = zone
		}
	}
	add("Europe/London", "ldn", "london", "uk", "gmt", "bst")
	add("America/Los_Angeles", "sf", "sfo", "san francisco", "la", "los angeles", "pt", "pst", "pdt")
	add("America/New_York", "nyc", "new york", "ny", "et", "est", "edt")
	add("Asia/Tokyo", "tokyo", "jst", "jp")
	add("Asia/Dubai", "dubai", "uae")
	add("Europe/Madrid", "madrid", "spain", "cet", "cest")
	add("Europe/Paris", "paris", "france")
	add("Europe/Berlin", "berlin", "germany")
	add("UTC", "utc", "z")
	return aliases
}

// normalizeZoneAlias lower-cases a zone name and turns dots, underscores and hyphens into spaces.
func normalizeZoneAlias(alias string) string {
	lowered := strings.ToLower(strings.TrimSpace(alias))
	return collapseSpaces(strings.Map(func(r rune) rune {
		if r == '.' || r == '_' || r == '-' {
			return ' '
		}
		return r
	}, lowered))
}

// zoneCache remembers zone lookups, including failed ones, so each name reads the zone database once.
type zoneCache struct {
	mu    sync.Mutex
	zones map[string]*time.Location
}

// load returns the location for an IANA name or alias target, caching the outcome. A nil location caches a
// failure.
func (c *zoneCache) load(name string) (*time.Location, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.zones == nil {
		c.zones = map[string]*time.Location{}
	}
	if loc, seen := c.zones[name]; seen {
		return loc, loc != nil
	}
	loc := loadZone(name)
	c.zones[name] = loc
	return loc, loc != nil
}

// loadZone reads a named zone from the zone database. An empty or unknown name yields nil.
func loadZone(name string) *time.Location {
	if name == "" {
		return nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil
	}
	return loc
}

// zoneForAlias resolves a city alias or an IANA name to its canonical name and location.
func (c *zoneCache) zoneForAlias(alias string) (string, *time.Location, bool) {
	trimmed := strings.TrimSpace(alias)
	if zone, known := zoneAliases[normalizeZoneAlias(trimmed)]; known {
		loc, ok := c.load(zone)
		return zone, loc, ok
	}
	loc, ok := c.load(trimmed)
	return trimmed, loc, ok
}

// parseClockHour converts a clock hour with an optional am or pm into a 24-hour hour.
func parseClockHour(hour int, meridiem string) (int, bool) {
	if hour < 0 || hour > 23 {
		return 0, false
	}
	if meridiem == "" {
		return hour, true
	}
	if hour < 1 || hour > 12 {
		return 0, false
	}
	if strings.ToLower(meridiem) == "am" {
		if hour == 12 {
			return 0, true
		}
		return hour, true
	}
	if hour == 12 {
		return 12, true
	}
	return hour + 12, true
}

// instantForZonedTime finds the instant at which a zone's wall clock shows the given local time. Local times that
// the zone skips, such as a spring-forward gap, have no instant and are refused.
func instantForZonedTime(loc *time.Location, year int, month time.Month, day, hour, minute int) (time.Time, bool) {
	instant := time.Date(year, month, day, hour, minute, 0, 0, loc)
	local := instant.In(loc)
	if local.Year() != year || local.Month() != month || local.Day() != day || local.Hour() != hour || local.Minute() != minute {
		return time.Time{}, false
	}
	return instant, true
}

// formatClock renders the 24-hour clock time of an instant in a zone, such as "21:00".
func formatClock(instant time.Time, loc *time.Location) string {
	return instant.In(loc).Format("15:04")
}

// calculateTimezone answers "time in <zone>" and "<hour> <zone> in <zone>". It returns false for anything else.
func calculateTimezone(zones *zoneCache, expression, query string, now time.Time) (Result, bool) {
	trimmed := strings.TrimSpace(expression)
	if groups := currentTimePattern.FindStringSubmatch(trimmed); groups != nil {
		name, loc, ok := zones.zoneForAlias(groups[1])
		if !ok {
			return Result{}, false
		}
		return timezoneResult(query, trimmed, now, formatClock(now, loc), name), true
	}
	groups := conversionTimePattern.FindStringSubmatch(trimmed)
	if groups == nil {
		return Result{}, false
	}
	_, sourceLoc, sourceOK := zones.zoneForAlias(groups[4])
	targetName, targetLoc, targetOK := zones.zoneForAlias(groups[5])
	if !sourceOK || !targetOK {
		return Result{}, false
	}
	hour, ok := parseClockHour(parseSmallInt(groups[1]), groups[3])
	if !ok {
		return Result{}, false
	}
	minute := 0
	if groups[2] != "" {
		minute = parseSmallInt(groups[2])
	}
	if minute > 59 {
		return Result{}, false
	}
	sourceDay := now.In(sourceLoc)
	instant, ok := instantForZonedTime(sourceLoc, sourceDay.Year(), sourceDay.Month(), sourceDay.Day(), hour, minute)
	if !ok {
		return Result{}, false
	}
	return timezoneResult(query, trimmed, instant, formatClock(instant, targetLoc), targetName), true
}

// timezoneResult builds a clock answer in a named zone.
func timezoneResult(query, expression string, instant time.Time, clock, zone string) Result {
	r := newResult(KindTimezone, strings.TrimSpace(query), strings.TrimSpace(expression), float64(instant.UnixMilli()), clock, clock, "Time in "+zone)
	r.Timezone = zone
	return describe(r)
}
