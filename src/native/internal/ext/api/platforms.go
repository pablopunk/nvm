package api

import (
	"slices"
	"strings"
)

// Operating system names, as runtime.GOOS spells them. System().OS() returns the same names.
const (
	OSDarwin  = "darwin"
	OSLinux   = "linux"
	OSWindows = "windows"
)

// Platforms is a set of operating systems an operation or an extension works on. The nil set is every OS. A set that
// is empty but not nil is no OS at all, which is what two operations that share no OS add up to.
type Platforms []string

// OnlyOn makes the set of the named operating systems.
func OnlyOn(oses ...string) Platforms {
	return append(Platforms{}, oses...)
}

// Includes reports whether the set holds goos. The nil set holds every OS.
func (p Platforms) Includes(goos string) bool {
	return p == nil || slices.Contains(p, goos)
}

// Everywhere reports whether the set is not limited to some OSes.
func (p Platforms) Everywhere() bool {
	return p == nil
}

// Intersect returns the OSes both sets hold. The nil set is the identity, so an operation that works everywhere
// narrows nothing.
func (p Platforms) Intersect(other Platforms) Platforms {
	if p == nil {
		return other
	}
	if other == nil {
		return p
	}
	both := Platforms{}
	for _, goos := range p {
		if slices.Contains(other, goos) {
			both = append(both, goos)
		}
	}
	return both
}

// Label says where the set works, in words for a listing: "macOS only", "macOS and Linux only", "no supported OS".
// Every OS is "".
func (p Platforms) Label() string {
	if p == nil {
		return ""
	}
	if len(p) == 0 {
		return "no supported OS"
	}
	return p.Names() + " only"
}

// Names lists the OSes of the set in words: "macOS", "macOS and Linux".
func (p Platforms) Names() string {
	names := make([]string, len(p))
	for i, goos := range p {
		names[i] = OSName(goos)
	}
	return strings.Join(names, " and ")
}

// OSName is the name of an operating system as its users say it.
func OSName(goos string) string {
	switch goos {
	case OSDarwin:
		return "macOS"
	case OSLinux:
		return "Linux"
	case OSWindows:
		return "Windows"
	}
	return goos
}

// PlatformOperation is one operation of the Context that exists on some operating systems only. The table below is
// the single place that says which: the host refuses the call elsewhere with ErrUnsupported, the extension manager
// does not run an extension whose source calls it on an OS that lacks it, and the AI builder's reference lists it.
// Nothing about it is declared in the manifest or chosen by the user.
type PlatformOperation struct {
	// Call is the chain of Context methods a source calls to reach the operation, such as Desktop, Shell, AppleScript.
	Call []string
	// Service and Op are the wire names the host serves the operation under (package protocol's ServiceDesktopShell
	// and OpAppleScript). A test keeps them in step with that package.
	Service string
	Op      string
	// Platforms are the OSes the operation exists on.
	Platforms Platforms
	// Note says why, for the builder's reference.
	Note string
}

// Name is the operation as a source writes it, without the context variable: Desktop().Shell().AppleScript.
func (o PlatformOperation) Name() string {
	return strings.Join(o.Call, "().")
}

// PlatformOperations lists every operation that exists on some operating systems only. An operation that is absent
// here works everywhere, perhaps with a reason of "unavailable" for a missing tool or permission. Add an entry
// whenever a service gains an operation that only some OS offers, and document it on the method.
var PlatformOperations = []PlatformOperation{
	{
		Call: []string{"Desktop", "Shell", "AppleScript"}, Service: "desktop.shell", Op: "apple-script",
		Platforms: OnlyOn(OSDarwin),
		Note:      "AppleScript runs through osascript, which only macOS has.",
	},
	{
		Call: []string{"Desktop", "WindowManagement", "SwitchDesktop"}, Service: "desktop.windows", Op: "switch-desktop",
		Platforms: OnlyOn(OSDarwin),
		Note:      "Switching desktops sends the Mission Control keys through System Events.",
	},
	{
		Call: []string{"Desktop", "Apps", "Hide"}, Service: "desktop.apps", Op: "hide-app",
		Platforms: OnlyOn(OSDarwin),
		Note:      "Only macOS can hide an app's windows.",
	},
	{
		Call: []string{"Desktop", "Keyboard", "TypeText"}, Service: "desktop.keyboard", Op: "type-text",
		Platforms: OnlyOn(OSDarwin),
		Note:      "Typing text as key presses is the keyboard.type-text capability, which only macOS offers.",
	},
}

// PlatformOperationOf finds the platform-specific operation the host serves under service and op.
func PlatformOperationOf(service, op string) (PlatformOperation, bool) {
	for _, operation := range PlatformOperations {
		if operation.Service == service && operation.Op == op {
			return operation, true
		}
	}
	return PlatformOperation{}, false
}
