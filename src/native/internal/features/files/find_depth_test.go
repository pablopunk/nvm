package files

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestFindDepthCountsFolderLevelsBelowTheRoot checks that depth zero reads only the items directly in a root, as 1.x did,
// and that each extra level reads one folder deeper.
func TestFindDepthCountsFolderLevelsBelowTheRoot(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys,
		filepath.Join(testHome, "Shots/root.png"),
		filepath.Join(testHome, "Shots/one/inner.png"),
		filepath.Join(testHome, "Shots/one/two/deep.png"),
	)
	for depth, want := range map[int][]string{
		0: {"root.png"},
		1: {"inner.png", "root.png"},
		2: {"deep.png", "inner.png", "root.png"},
	} {
		result, err := Find(context.Background(), fsys, []string{"~/Shots"}, FindOptions{
			Home:  testHome,
			Depth: depth,
			Query: Query{SortBy: SortName, Order: OrderAsc},
		})
		if err != nil {
			t.Fatalf("depth %d: %v", depth, err)
		}
		if got := namesOf(result.Results); !slices.Equal(got, want) {
			t.Fatalf("depth %d found %v; want %v", depth, got, want)
		}
	}
}
