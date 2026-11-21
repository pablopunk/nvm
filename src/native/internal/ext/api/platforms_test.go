package api

import "testing"

// TestPlatformOperationsAreServedUnderDistinctNames: a call finds at most one entry, by chain and by wire name.
func TestPlatformOperationsAreServedUnderDistinctNames(t *testing.T) {
	names, wires := map[string]bool{}, map[string]bool{}
	for _, operation := range PlatformOperations {
		wire := operation.Service + "/" + operation.Op
		if names[operation.Name()] || wires[wire] {
			t.Errorf("%s is listed twice", operation.Name())
		}
		names[operation.Name()], wires[wire] = true, true
		if operation.Platforms.Everywhere() || operation.Note == "" || len(operation.Call) == 0 {
			t.Errorf("%s must name the OSes it exists on and say why", operation.Name())
		}
		if found, ok := PlatformOperationOf(operation.Service, operation.Op); !ok || found.Name() != operation.Name() {
			t.Errorf("%s is not found by its wire name", operation.Name())
		}
	}
}

// TestPlatformOperationsAreDesktopOperations: the preloaded builder reference lists them under one Desktop() header.
func TestPlatformOperationsAreDesktopOperations(t *testing.T) {
	for _, operation := range PlatformOperations {
		if operation.Call[0] != "Desktop" {
			t.Errorf("%s is not a Desktop() operation; list its full name in the builder's terse reference", operation.Name())
		}
	}
}
