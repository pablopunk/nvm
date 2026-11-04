package shortcuts

import (
	"cmp"
	"slices"
	"strings"
)

// Source says where a binding comes from: the user's choice, or a shortcut an extension declares.
type Source int

// Binding sources. A user choice beats a declared shortcut for the same action and accelerator.
const (
	SourceUser Source = iota
	SourceDeclared
)

// Binding is a global shortcut the user or an extension wants: the action, its accelerator, and the title that
// conflict messages name the action by.
type Binding struct {
	ActionID    string
	Accelerator string
	Title       string
	Source      Source
}

// Declared is a shortcut an extension asks for. It applies only while the user has not chosen one for the action.
type Declared struct {
	ActionID    string
	Accelerator string
	Title       string
}

// Claim is a binding the plan registers.
type Claim struct {
	ActionID    string
	Accelerator string
}

// Inactive is a wanted binding the plan does not register, with the message the user sees under it.
type Inactive struct {
	ActionID    string
	Accelerator string
	Message     string
}

// Plan is the registrations to make and the wanted bindings that stay inactive.
type Plan struct {
	Claims   []Claim
	Inactive []Inactive
}

// Wanted lists the bindings the user's choices and the declared shortcuts ask for. Actions missing from titles are
// dormant (their command is gone) and are skipped. A declared shortcut is skipped when the user has chosen for its
// action, or has removed it.
func Wanted(actions map[string]string, disabled map[string]bool, titles map[string]string, declared []Declared) []Binding {
	var wanted []Binding
	for _, actionID := range sortedKeys(actions) {
		title, known := titles[actionID]
		if !known {
			continue
		}
		wanted = append(wanted, Binding{ActionID: actionID, Accelerator: actions[actionID], Title: title, Source: SourceUser})
	}
	for _, item := range declared {
		if _, chosen := actions[item.ActionID]; chosen || disabled[item.ActionID] {
			continue
		}
		wanted = append(wanted, Binding{ActionID: item.ActionID, Accelerator: Normalize(item.Accelerator), Title: item.Title, Source: SourceDeclared})
	}
	return wanted
}

// PlanBindings decides which wanted bindings to register. Each accelerator goes to one action: user choices come
// before declared shortcuts, then the action IDs decide, so the plan does not depend on map order. An accelerator
// that is reserved, or already claimed, leaves the binding inactive with the reason.
func PlanBindings(wanted []Binding, reserved func(accelerator string) bool) Plan {
	var plan Plan
	claimed := map[string]string{}
	for _, binding := range sortedByClaimOrder(wanted) {
		switch owner, taken := claimed[binding.Accelerator]; {
		case reserved(binding.Accelerator):
			plan.Inactive = append(plan.Inactive, Inactive{binding.ActionID, binding.Accelerator, binding.Accelerator + " is reserved for opening Nevermind"})
		case taken:
			plan.Inactive = append(plan.Inactive, Inactive{binding.ActionID, binding.Accelerator, binding.Accelerator + " is already used by " + owner})
		default:
			claimed[binding.Accelerator] = binding.Title
			plan.Claims = append(plan.Claims, Claim{binding.ActionID, binding.Accelerator})
		}
	}
	return plan
}

func sortedByClaimOrder(wanted []Binding) []Binding {
	ordered := slices.Clone(wanted)
	slices.SortStableFunc(ordered, func(a, b Binding) int {
		return cmp.Or(cmp.Compare(a.Source, b.Source), strings.Compare(a.ActionID, b.ActionID))
	})
	return ordered
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
