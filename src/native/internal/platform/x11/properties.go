package x11

import (
	"strconv"
	"strings"
)

// Rect is a rectangle in X screen pixels. Origins may be negative for monitors left of or above the origin.
type Rect struct {
	X, Y, Width, Height int
}

// Intersect returns the overlap of two rectangles and whether it is non-empty.
func (r Rect) Intersect(other Rect) (Rect, bool) {
	left := max(r.X, other.X)
	top := max(r.Y, other.Y)
	right := min(r.X+r.Width, other.X+other.Width)
	bottom := min(r.Y+r.Height, other.Y+other.Height)
	if right <= left || bottom <= top {
		return Rect{}, false
	}
	return Rect{X: left, Y: top, Width: right - left, Height: bottom - top}, true
}

// Properties is the output of one xprop call on a window, keyed by property name. Values keep the
// printed form (quoted strings stay quoted); the typed accessors below decode them.
type Properties map[string]string

// ParseProperties reads xprop output lines of the form `NAME(TYPE) = value` and `NAME: not found.`.
// A property that is not set maps to the empty string.
func ParseProperties(output string) Properties {
	properties := Properties{}
	for _, line := range strings.Split(output, "\n") {
		name, value, found := strings.Cut(line, " = ")
		if !found {
			if name, _, ok := strings.Cut(line, ": not found"); ok {
				properties[propertyName(name)] = ""
			}
			continue
		}
		properties[propertyName(name)] = strings.TrimSpace(value)
	}
	return properties
}

// propertyName drops the printed type from `NAME(TYPE)`.
func propertyName(printed string) string {
	name, _, _ := strings.Cut(strings.TrimSpace(printed), "(")
	return name
}

// Cardinal decodes a CARDINAL value such as `91819`. It reports false when the value is missing.
func (p Properties) Cardinal(name string) (int, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(p[name]))
	return value, err == nil
}

// Cardinals decodes a comma-separated CARDINAL list such as `1, 1, 20, 1`. A missing property yields nil.
func (p Properties) Cardinals(name string) []int {
	return cardinals(p[name])
}

// String decodes the first quoted string of a value such as `"xfce4-panel", "Xfce4-panel"` or a
// UTF8_STRING title, and falls back to the raw value when it is not quoted.
func (p Properties) String(name string) string {
	return firstQuoted(p[name])
}

// WMClass returns the class part of WM_CLASS (the second string), which names the application.
// Some windows set only an instance name; the first string is used then.
func (p Properties) WMClass() string {
	strs := quotedStrings(p["WM_CLASS"])
	switch len(strs) {
	case 0:
		return ""
	case 1:
		return strs[0]
	}
	return strs[1]
}

// Atoms returns the atom names of a list property such as _NET_WM_STATE, or nil when it is empty.
func (p Properties) Atoms(name string) []string {
	value := strings.TrimSpace(p[name])
	if value == "" {
		return nil
	}
	return strings.Split(value, ", ")
}

// HasAtom reports whether an atom list property contains atom.
func (p Properties) HasAtom(name, atom string) bool {
	for _, candidate := range p.Atoms(name) {
		if candidate == atom {
			return true
		}
	}
	return false
}

func firstQuoted(value string) string {
	if strs := quotedStrings(value); len(strs) > 0 {
		return strs[0]
	}
	return strings.TrimSpace(value)
}

// quotedStrings decodes a comma-separated list of Go-style quoted strings. Values that are not quoted are
// skipped, so unexpected output yields fewer strings instead of a wrong one.
func quotedStrings(value string) []string {
	var out []string
	rest := strings.TrimSpace(value)
	for strings.HasPrefix(rest, "\"") {
		end := closingQuote(rest)
		if end < 0 {
			break
		}
		if decoded, err := strconv.Unquote(rest[:end+1]); err == nil {
			out = append(out, decoded)
		}
		rest = strings.TrimLeft(rest[end+1:], ", ")
	}
	return out
}

// closingQuote returns the index of the quote that closes the string at the start of s, honouring
// backslash escapes, or -1.
func closingQuote(s string) int {
	for index := 1; index < len(s); index++ {
		switch s[index] {
		case '\\':
			index++
		case '"':
			return index
		}
	}
	return -1
}
