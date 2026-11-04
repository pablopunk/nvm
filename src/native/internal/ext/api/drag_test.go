package api

import "testing"

// TestDragPathsTakesTheFileThenTheFirstActionPath checks the 1.x rule: an item's absolute file drags, else the first
// action that names an absolute path, and nothing drags when no path is absolute.
func TestDragPathsTakesTheFileThenTheFirstActionPath(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want []string
	}{
		{"file", Item{File: &FileRef{Path: "/a/b.png"}}, []string{"/a/b.png"}},
		{"open-path primary", Item{Primary: ptrAction(OpenPath("Open", "/home/u/Downloads"))}, []string{"/home/u/Downloads"}},
		{"action panel", Item{ActionPanel: &ActionPanel{Sections: []ActionSection{{Actions: []Action{OpenPath("Open", "/x")}}}}}, []string{"/x"}},
		{"relative file", Item{File: &FileRef{Path: "b.png"}}, nil},
		{"relative action", Item{Primary: ptrAction(OpenPath("Open", "x"))}, nil},
		{"no path", Item{Title: "Plain"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DragPaths(c.item)
			if len(got) != len(c.want) || (len(got) > 0 && got[0] != c.want[0]) {
				t.Fatalf("DragPaths = %v, want %v", got, c.want)
			}
		})
	}
}

// TestDragImagePathNamesTheDragPicture checks the optional drag picture: it is the DragImage path, and empty when none.
func TestDragImagePathNamesTheDragPicture(t *testing.T) {
	if got := DragImagePath(Item{File: &FileRef{Path: "/a.mov"}, DragImage: &FileRef{Path: "/a.png"}}); got != "/a.png" {
		t.Fatalf("DragImagePath = %q, want the drag picture", got)
	}
	if got := DragImagePath(Item{File: &FileRef{Path: "/a.png"}}); got != "" {
		t.Fatalf("DragImagePath = %q, want none", got)
	}
}

// TestValidateItemsRejectsABadDragImage checks the API validates the new field: a drag picture needs a file to drag
// and an absolute path, and a valid one passes.
func TestValidateItemsRejectsABadDragImage(t *testing.T) {
	cases := []struct {
		name    string
		item    Item
		wantErr bool
	}{
		{"valid", Item{ID: "a", File: &FileRef{Path: "/a.mov"}, DragImage: &FileRef{Path: "/a.png"}}, false},
		{"no file", Item{ID: "a", DragImage: &FileRef{Path: "/a.png"}}, true},
		{"relative picture", Item{ID: "a", File: &FileRef{Path: "/a.mov"}, DragImage: &FileRef{Path: "a.png"}}, true},
		{"no picture", Item{ID: "a", File: &FileRef{Path: "/a.mov"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateItems([]Item{c.item})
			if (err != nil) != c.wantErr {
				t.Fatalf("validateItems error = %v, want error %v", err, c.wantErr)
			}
		})
	}
}

func ptrAction(a Action) *Action { return &a }
