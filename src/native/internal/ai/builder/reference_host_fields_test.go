package builder

import (
	"strings"
	"testing"
)

func TestReferenceOmitsHostOnlySnapshotFields(t *testing.T) {
	for _, reference := range []string{apiReference, apiReferenceCompact} {
		for _, field := range []string{"ProtectSnapshot", "OnSnapshotDenied", "SnapshotEvidence", "SnapshotDenial"} {
			if strings.Contains(reference, field) {
				t.Errorf("reference exposes host-only %s", field)
			}
		}
	}
}
