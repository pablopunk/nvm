package api

import "testing"

// TestConvertedFromIsAPlain1xSourceName: convertedFrom names one 1.x .ts file in the shared folder; anything else would
// let a manifest point the inheritance at a path outside it.
func TestConvertedFromIsAPlain1xSourceName(t *testing.T) {
	manifest := func(from string) Manifest {
		return Manifest{APIVersion: "1.0", Identity: Identity{ID: "acme.theme", Title: "Acme Theme"}, ConvertedFrom: from}
	}
	for _, ok := range []string{"", "toggle-system-theme-257c802c.ts", "caffeinate.ts"} {
		if err := manifest(ok).Validate(); err != nil {
			t.Errorf("convertedFrom %q: %v", ok, err)
		}
	}
	for _, bad := range []string{"caffeinate.js", "../state.ts", "sub/caffeinate.ts", `sub\caffeinate.ts`, ".hidden.ts"} {
		if err := manifest(bad).Validate(); err == nil {
			t.Errorf("convertedFrom %q was accepted", bad)
		}
	}
}
