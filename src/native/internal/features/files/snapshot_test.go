package files

import (
	"slices"
	"testing"
	"unsafe"

	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

// TestEntryIsFortyBytes keeps the per-file cost of the index from growing by accident.
func TestEntryIsFortyBytes(t *testing.T) {
	if size := unsafe.Sizeof(entry{}); size != entrySize {
		t.Fatalf("entry is %d bytes, want %d", size, entrySize)
	}
}

// TestBuilderInternsDirectoriesAndExtensions checks that a folder path and an extension are stored once per snapshot.
func TestBuilderInternsDirectoriesAndExtensions(t *testing.T) {
	b := newBuilder(4)
	for _, name := range []string{"a.txt", "b.TXT", "c.png"} {
		if !b.addRecord(record{dir: testpath.Abs("home", "me", "Documents"), name: name, size: 3, modMs: 7}) {
			t.Fatalf("addRecord(%s) refused", name)
		}
	}
	snap := b.build(1, "key")
	if len(snap.dirs) != 1 || snap.dirs[0] != testpath.Abs("home", "me", "Documents") {
		t.Fatalf("dirs = %v, want the folder once", snap.dirs)
	}
	if !slices.Equal(snap.exts, []string{"txt", "png"}) {
		t.Fatalf("exts = %v, want lowercased and deduplicated", snap.exts)
	}
	if snap.Len() != 3 || snap.Path(1) != testpath.Abs("home", "me", "Documents", "b.TXT") || snap.Name(2) != "c.png" {
		t.Fatalf("entries read back wrong: %d files, %q, %q", snap.Len(), snap.Path(1), snap.Name(2))
	}
	if info := snap.Info(2); info.Kind != KindImage || info.Extension != "png" || info.Size != 3 {
		t.Fatalf("Info(2) = %+v", info)
	}
}

// TestBuilderRefusesNamesItCannotRepresent checks the empty and oversized names are skipped, not truncated.
func TestBuilderRefusesNamesItCannotRepresent(t *testing.T) {
	b := newBuilder(0)
	if b.addRecord(record{dir: testpath.Abs("d"), name: ""}) {
		t.Error("an empty name must be refused")
	}
	long := make([]byte, maxNameBytes+1)
	for i := range long {
		long[i] = 'x'
	}
	if b.addRecord(record{dir: testpath.Abs("d"), name: string(long)}) {
		t.Error("a name over the limit must be refused, not truncated")
	}
	if snap := b.build(1, ""); snap.Len() != 0 {
		t.Fatalf("refused names left %d entries behind", snap.Len())
	}
}

// TestDeltaDropsSubtreesAndFilesAndAddsRecords checks the copy-on-write rebuild: the old snapshot keeps its content, the
// new one reflects the delta.
func TestDeltaDropsSubtreesAndFilesAndAddsRecords(t *testing.T) {
	base := newBuilder(8)
	for _, rec := range []record{
		{dir: testpath.Abs("r"), name: "top.txt"},
		{dir: testpath.Abs("r", "sub"), name: "a.txt"},
		{dir: testpath.Abs("r", "sub", "deep"), name: "b.txt"},
		{dir: testpath.Abs("r", "other"), name: "c.txt"},
		{dir: testpath.Abs("r", "other"), name: "gone.txt"},
	} {
		base.addRecord(rec)
	}
	old := base.build(1, "k")

	d := newDelta()
	d.dropSubtree(testpath.Abs("r", "sub"))
	d.dropFile(testpath.Abs("r", "other"), "gone.txt")
	d.addRecords([]record{{dir: testpath.Abs("r", "sub"), name: "fresh.txt"}})
	next := d.apply(old, 2, "k")

	if old.Len() != 5 {
		t.Fatalf("the old snapshot changed: %d entries", old.Len())
	}
	got := pathsOf(next)
	want := []string{testpath.Abs("r", "top.txt"), testpath.Abs("r", "other", "c.txt"), testpath.Abs("r", "sub", "fresh.txt")}
	if !slices.Equal(sortedCopy(got), sortedCopy(want)) {
		t.Fatalf("paths after the delta = %v, want %v", got, want)
	}
	if next.Generation() != 2 {
		t.Fatalf("generation = %d, want 2", next.Generation())
	}
}

// TestDeltaKeepsFoldersThatTheScanCouldNotRead checks that a kept folder survives a drop of its ancestor.
func TestDeltaKeepsFoldersThatTheScanCouldNotRead(t *testing.T) {
	base := newBuilder(4)
	base.addRecord(record{dir: testpath.Abs("r", "locked"), name: "keep.txt"})
	base.addRecord(record{dir: testpath.Abs("r", "open"), name: "drop.txt"})
	old := base.build(1, "k")

	d := newDelta()
	d.dropSubtree(testpath.Abs("r"))
	d.keepDirs = []string{testpath.Abs("r", "locked")}
	next := d.apply(old, 2, "k")

	if got := pathsOf(next); !slices.Equal(got, []string{testpath.Abs("r", "locked", "keep.txt")}) {
		t.Fatalf("after the drop = %v, want only the kept folder", got)
	}
}

// TestDeltaRemapsTablesOfTheOldSnapshot checks that carried-over entries keep their names, folders and extensions.
func TestDeltaRemapsTablesOfTheOldSnapshot(t *testing.T) {
	base := newBuilder(4)
	base.addRecord(record{dir: testpath.Abs("x"), name: "one.pdf", size: 11})
	base.addRecord(record{dir: testpath.Abs("y"), name: "two.png", size: 22})
	old := base.build(1, "k")

	d := newDelta()
	d.dropSubtree(testpath.Abs("x"))
	next := d.apply(old, 2, "k")

	if next.Len() != 1 || next.Path(0) != testpath.Abs("y", "two.png") {
		t.Fatalf("after dropping /x = %d entries, first %q", next.Len(), next.Path(0))
	}
	if info := next.Info(0); info.Size != 22 || info.Kind != KindImage || info.Extension != "png" {
		t.Fatalf("carried metadata = %+v", info)
	}
}

// sortedCopy returns a sorted copy of paths, so set comparisons ignore order.
func sortedCopy(paths []string) []string {
	out := slices.Clone(paths)
	slices.Sort(out)
	return out
}
