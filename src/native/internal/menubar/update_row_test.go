package menubar

import "testing"

func TestUpdateRowSitsUnderOpenWithADivider(t *testing.T) {
	rows := RowsWithUpdate(Actions{}, nil, &UpdateRow{Label: "Restart to Update"})
	if rows[0].ID != "open" || rows[2].ID != "update" || rows[2].Label != "Restart to Update" || !rows[3].Separator {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[4].ID != "check-updates" {
		t.Fatalf("the rest of the menu moved: %+v", rows[4])
	}
}

func TestMenuWithoutAnUpdateIsUnchanged(t *testing.T) {
	plain := Rows(Actions{}, nil)
	with := RowsWithUpdate(Actions{}, nil, nil)
	if len(plain) != len(with) {
		t.Fatalf("menu length %d, want %d", len(with), len(plain))
	}
}
