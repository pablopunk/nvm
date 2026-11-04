package extjobs

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestFileFilterAppliesHiddenExtensionIgnoreAndKind(t *testing.T) {
	cases := []struct {
		name    string
		trigger api.Trigger
		path    string
		want    bool
	}{
		{"plain match", api.Trigger{}, "/d/a.txt", true},
		{"hidden skipped", api.Trigger{}, "/d/.a.txt", false},
		{"hidden included", api.Trigger{IncludeHidden: true}, "/d/.a.txt", true},
		{"extension match ignores case and dot", api.Trigger{Extensions: []string{".PDF"}}, "/d/a.pdf", true},
		{"extension mismatch", api.Trigger{Extensions: []string{"pdf"}}, "/d/a.txt", false},
		{"ignore pattern", api.Trigger{Ignore: []string{"*.tmp"}}, "/d/a.tmp", false},
		{"image kind", api.Trigger{FileKind: api.FileKindImage}, "/d/a.heic", true},
		{"image kind rejects video", api.Trigger{FileKind: api.FileKindImage}, "/d/a.mp4", false},
		{"media kind takes video", api.Trigger{FileKind: api.FileKindMedia}, "/d/a.mp4", true},
		{"file kind narrows nothing", api.Trigger{FileKind: api.FileKindFile}, "/d/a.mp4", true},
	}
	for _, tc := range cases {
		if got := newFileFilter(tc.trigger).matches(tc.path); got != tc.want {
			t.Errorf("%s: matches(%s) = %v, want %v", tc.name, tc.path, got, tc.want)
		}
	}
}

func TestExpandRootsKeepsOnlyAbsoluteRootsAndExpandsHome(t *testing.T) {
	got := expandRoots([]string{"~/Desktop", "/tmp/x/", "relative", "~"}, "/home/acme")
	want := []string{"/home/acme/Desktop", "/tmp/x", "/home/acme"}
	if len(got) != len(want) {
		t.Fatalf("roots = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("roots = %v, want %v", got, want)
		}
	}
}
