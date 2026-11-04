package x11

import (
	"regexp"
	"strconv"
	"strings"
)

// WindowIDs parses a window list property such as `_NET_CLIENT_LIST_STACKING(WINDOW): window id # 0x1, 0x2`.
// Windows are returned in the printed order. An unset property yields nil.
func WindowIDs(output string) []uint32 {
	_, list, found := strings.Cut(output, "# ")
	if !found {
		return nil
	}
	var ids []uint32
	for _, field := range strings.Split(list, ",") {
		id, err := strconv.ParseUint(strings.TrimSpace(field), 0, 32)
		if err == nil {
			ids = append(ids, uint32(id))
		}
	}
	return ids
}

// WorkArea parses `_NET_WORKAREA(CARDINAL) = x, y, w, h, ...` and returns the first desktop's area.
func WorkArea(output string) (Rect, bool) {
	_, values, found := strings.Cut(output, " = ")
	if !found {
		return Rect{}, false
	}
	numbers := cardinals(values)
	if len(numbers) < 4 {
		return Rect{}, false
	}
	return Rect{X: numbers[0], Y: numbers[1], Width: numbers[2], Height: numbers[3]}, true
}

func cardinals(values string) []int {
	var numbers []int
	for _, field := range strings.Split(values, ",") {
		if number, err := strconv.Atoi(strings.TrimSpace(field)); err == nil {
			numbers = append(numbers, number)
		}
	}
	return numbers
}

// Geometry parses xwininfo output for one window: its absolute upper-left corner and its size.
func Geometry(output string) (Rect, bool) {
	fields := map[string]int{}
	for _, line := range strings.Split(output, "\n") {
		label, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found {
			continue
		}
		number, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil {
			fields[strings.TrimSpace(label)] = number
		}
	}
	x, hasX := fields["Absolute upper-left X"]
	y, hasY := fields["Absolute upper-left Y"]
	width, hasWidth := fields["Width"]
	height, hasHeight := fields["Height"]
	if !(hasX && hasY && hasWidth && hasHeight) {
		return Rect{}, false
	}
	return Rect{X: x, Y: y, Width: width, Height: height}, true
}

// Monitor is one connected output as xrandr reports it.
type Monitor struct {
	Name    string
	Primary bool
	Bounds  Rect
}

var monitorGeometry = regexp.MustCompile(`(\d+)x(\d+)([+-]-?\d+)([+-]-?\d+)`)

// Monitors parses `xrandr --query` output and returns the connected outputs with their current modes.
func Monitors(output string) []Monitor {
	var monitors []Monitor
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "connected" {
			continue
		}
		bounds, ok := monitorBounds(line)
		if !ok {
			continue
		}
		monitors = append(monitors, Monitor{
			Name:    fields[0],
			Primary: containsField(fields, "primary"),
			Bounds:  bounds,
		})
	}
	return monitors
}

func monitorBounds(line string) (Rect, bool) {
	match := monitorGeometry.FindStringSubmatch(line)
	if match == nil {
		return Rect{}, false
	}
	width, errW := strconv.Atoi(match[1])
	height, errH := strconv.Atoi(match[2])
	x, errX := strconv.Atoi(match[3])
	y, errY := strconv.Atoi(match[4])
	if errW != nil || errH != nil || errX != nil || errY != nil {
		return Rect{}, false
	}
	return Rect{X: x, Y: y, Width: width, Height: height}, true
}

func containsField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}
