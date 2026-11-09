package builder

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// undiscoveredTargets are ledger targets the builder's reference leaves out on purpose. Entries can only be removed.
var undiscoveredTargets = map[string]string{}

type discoveryRow struct {
	Member string `json:"member"`
	Status string `json:"status"`
	Target string `json:"target"`
}

// referenceSections maps each type heading of the reference to the lowercased text under it.
func referenceSections(reference string) map[string]string {
	sections := map[string]string{}
	current := ""
	for _, line := range strings.Split(reference, "\n") {
		if strings.HasPrefix(line, "#") {
			current = strings.TrimSpace(strings.TrimLeft(line, "#"))
			continue
		}
		if current != "" {
			sections[current] += strings.ToLower(line) + "\n"
		}
	}
	return sections
}

func referenceNames(sections map[string]string, reference, target string) bool {
	kind, name, qualified := strings.Cut(target, ":")
	if qualified {
		if kind == "value" {
			return true
		}
		return strings.Contains(reference, name)
	}
	typeName, member, hasMember := strings.Cut(target, ".")
	section, known := sections[typeName]
	if !known {
		return false
	}
	return !hasMember || strings.Contains(section, strings.ToLower(member))
}

// TestReferenceNamesEverythingTheLedgerPromises is gate G3 of docs/extension-api-parity-plan.md: the builder can only
// use what its reference names, so every 1.x capability that survives must be findable there.
func TestReferenceNamesEverythingTheLedgerPromises(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/parity/api-ledger.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []discoveryRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	sections := referenceSections(apiReference)
	missing := map[string]bool{}
	for _, row := range rows {
		if row.Status == "dropped" || row.Status == "open" || referenceNames(sections, apiReference, row.Target) {
			continue
		}
		missing[row.Target] = true
	}
	for target := range missing {
		if _, allowed := undiscoveredTargets[target]; !allowed {
			t.Errorf("the reference does not name %s", target)
		}
	}
	for target := range undiscoveredTargets {
		if !missing[target] {
			t.Errorf("undiscoveredTargets lists %s, which the reference names now", target)
		}
	}
}
