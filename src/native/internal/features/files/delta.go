package files

// delta describes the difference between two snapshots: whole folder subtrees to drop, single files to drop and
// records to add. Applying a delta builds a new snapshot; the old one stays valid for the readers holding it.
type delta struct {
	// dropDirs lists folders whose entire subtree is removed, the folder itself included.
	dropDirs []string
	// keepDirs lists folders that stay as they were even when a dropDirs entry covers them. A scan uses it for
	// folders it could not read, so their previously indexed files are not lost.
	keepDirs []string
	// dropFiles lists the names removed from each folder.
	dropFiles map[string]map[string]bool
	add       []record
}

// newDelta returns an empty delta.
func newDelta() *delta {
	return &delta{dropFiles: map[string]map[string]bool{}}
}

// empty reports whether applying the delta would change nothing.
func (d *delta) empty() bool {
	return len(d.dropDirs) == 0 && len(d.dropFiles) == 0 && len(d.add) == 0
}

// dropSubtree removes a folder and everything under it.
func (d *delta) dropSubtree(dir string) {
	d.dropDirs = append(d.dropDirs, dir)
}

// dropFile removes one file name from a folder.
func (d *delta) dropFile(dir, name string) {
	names := d.dropFiles[dir]
	if names == nil {
		names = map[string]bool{}
		d.dropFiles[dir] = names
	}
	names[name] = true
}

// addRecords queues files to add.
func (d *delta) addRecords(records []record) {
	d.add = append(d.add, records...)
}

// dropMasks precomputes, per directory of old, whether the whole directory is dropped and which of its names
// are dropped, so the per-entry pass does no string work for untouched directories.
type dropMasks struct {
	subtree []bool
	names   []map[string]bool
}

// masksFor computes the masks of d against the directory table of old.
func (d *delta) masksFor(old *Snapshot) dropMasks {
	masks := dropMasks{
		subtree: make([]bool, len(old.dirs)),
		names:   make([]map[string]bool, len(old.dirs)),
	}
	for id, dir := range old.dirs {
		masks.subtree[id] = underAny(dir, d.dropDirs) && !underAny(dir, d.keepDirs)
		if names, ok := d.dropFiles[dir]; ok {
			masks.names[id] = names
		}
	}
	return masks
}

// apply builds the snapshot that results from old plus d. generation and configKey are carried onto it.
func (d *delta) apply(old *Snapshot, generation uint64, configKey string) *Snapshot {
	masks := d.masksFor(old)
	b := newBuilder(len(old.entries) + len(d.add))
	dirMap := filled(len(old.dirs), -1)
	extMap := filled(len(old.exts), -1)
	for _, e := range old.entries {
		if masks.subtree[e.dir] {
			continue
		}
		if names := masks.names[e.dir]; names != nil && names[string(old.nameOf(e))] {
			continue
		}
		b.appendFrom(old, e, dirMap, extMap)
	}
	for _, rec := range d.add {
		b.addRecord(rec)
	}
	return b.build(generation, configKey)
}

// underAny reports whether dir is one of folders or lies beneath one of them.
func underAny(dir string, folders []string) bool {
	for _, folder := range folders {
		if IsInsideRoot(dir, folder) {
			return true
		}
	}
	return false
}

// filled returns a slice of n values, all set to fill.
func filled(n int, fill int32) []int32 {
	out := make([]int32, n)
	for i := range out {
		out[i] = fill
	}
	return out
}
