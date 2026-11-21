package enablement

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestInheritTakesTheOneFirstStateAndKeepsALaterChoice: the converted folder is on from its 1.x file once. A later
// disable made in 2.0 is not overwritten by the file on the next scan.
func TestInheritTakesTheOneFirstStateAndKeepsALaterChoice(t *testing.T) {
	files := fake.NewFS()
	set, s := openSet(t, files)
	defer s.Close()
	if err := set.Inherit("acme.theme", "acme-theme.ts", true, nil); err != nil {
		t.Fatal(err)
	}
	if on, _ := set.Enabled("acme.theme", nil); !on {
		t.Fatal("the inherited on state was not applied")
	}
	if err := set.SetEnabled("acme.theme", false, nil); err != nil {
		t.Fatal(err)
	}
	if err := set.Inherit("acme.theme", "acme-theme.ts", true, nil); err != nil {
		t.Fatal(err)
	}
	if on, _ := set.Enabled("acme.theme", nil); on {
		t.Fatal("a second inheritance turned a disabled extension back on")
	}
	if done, _ := set.Inherited("acme.theme"); !done {
		t.Fatal("the inheritance is not recorded")
	}
}

// TestInheritingAnOffFileRecordsWithoutEnabling: a 1.x file that was off is recorded and leaves the folder off.
func TestInheritingAnOffFileRecordsWithoutEnabling(t *testing.T) {
	set, s := openSet(t, fake.NewFS())
	defer s.Close()
	if err := set.Inherit("acme.theme", "acme-theme.ts", false, nil); err != nil {
		t.Fatal(err)
	}
	if on, _ := set.Enabled("acme.theme", nil); on {
		t.Fatal("an off 1.x file turned the folder on")
	}
	if done, _ := set.Inherited("acme.theme"); !done {
		t.Fatal("an off 1.x file was not recorded")
	}
}
