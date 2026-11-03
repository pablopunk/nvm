package platform

import (
	"reflect"
	"slices"
	"testing"
)

func TestAllCapabilitiesAreSortedAndUnique(t *testing.T) {
	all := AllCapabilities()
	if !slices.IsSorted(all) {
		t.Fatalf("AllCapabilities is not sorted: %v", all)
	}
	for i := 1; i < len(all); i++ {
		if all[i] == all[i-1] {
			t.Fatalf("AllCapabilities repeats %q", all[i])
		}
	}
}

func TestCapabilitiesReportOnlyTheListedNames(t *testing.T) {
	set := NewCapabilities(CapabilityShellTrash, CapabilityClipboardImage)
	if !set.Supported(CapabilityShellTrash) || set.Supported(CapabilityShellReveal) {
		t.Fatal("Supported does not match the listed capabilities")
	}
	if set.Supported("made.up") {
		t.Fatal("an unknown capability name reported as supported")
	}
	want := []Capability{CapabilityClipboardImage, CapabilityShellTrash}
	if got := set.List(); !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
}

func TestZeroCapabilitiesSupportNothing(t *testing.T) {
	var none Capabilities
	for _, name := range AllCapabilities() {
		if none.Supported(name) {
			t.Fatalf("zero value supports %q", name)
		}
	}
	if len(none.List()) != 0 {
		t.Fatal("zero value lists capabilities")
	}
}
