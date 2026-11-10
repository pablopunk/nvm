package api

import (
	"encoding/json"
	"testing"
)

func TestOperationRequirementsRoundTripAndLegacyCompatibility(t *testing.T) {
	action := Run("preview", "Preview", nil)
	action.Requires = []Operation{OperationFilePreview}
	command := Command{ID: "preview", Title: "Preview", Requires: []Operation{OperationFilePreview}, Actions: []Action{action}}
	item := Item{ID: "preview", Title: "Preview", Requires: []Operation{OperationFilePreview}, Primary: &action}
	for _, value := range []any{command, item, action} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		var required []Operation
		if err := json.Unmarshal(fields["requires"], &required); err != nil || len(required) != 1 || required[0] != OperationFilePreview {
			t.Fatalf("requires lost in wire mapping: %s (%v)", data, err)
		}
	}
	for _, version := range []string{"1.0", "1.1"} {
		if err := CheckCompatibility(version, Current); err != nil {
			t.Fatal(err)
		}
	}
	if err := (Command{ID: "bad", Title: "Bad", Requires: []Operation{""}}).Validate(); err == nil {
		t.Fatal("empty requirement accepted")
	}
	if err := (Command{ID: "future", Title: "Future", Requires: []Operation{"future.operation"}}).Validate(); err != nil {
		t.Fatal("unknown operations must remain safe discovery metadata", err)
	}
}
