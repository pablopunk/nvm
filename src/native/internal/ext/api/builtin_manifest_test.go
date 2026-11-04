package api

import "testing"

func TestBuiltInManifestMayUseTheReservedPrefixAndFolderOnesMayNot(t *testing.T) {
	builtIn := Manifest{APIVersion: "1.0", Identity: Identity{ID: "nevermind.calculator", Title: "Calculator"}}
	if err := builtIn.ValidateBuiltIn(); err != nil {
		t.Fatalf("ValidateBuiltIn: %v", err)
	}
	if err := builtIn.Validate(); err == nil {
		t.Fatalf("Validate accepted the reserved prefix")
	}
	plain := Manifest{APIVersion: "1.0", Identity: Identity{ID: "acme.clock", Title: "Clock"}}
	if err := plain.ValidateBuiltIn(); err == nil {
		t.Fatalf("ValidateBuiltIn accepted an id outside the reserved prefix")
	}
}
