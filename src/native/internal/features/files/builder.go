package files

import "math"

// maxNameBytes bounds one file name. Longer names are skipped rather than truncated, because a truncated name
// would point at a different file.
const maxNameBytes = 4096

// maxExtensions is the number of distinct extensions a snapshot can hold; entries index them with 16 bits.
const maxExtensions = math.MaxUint16

// record is one file found by a scan or a targeted re-read, before it is packed into an entry.
type record struct {
	dir       string
	name      string
	size      int64
	modMs     int64
	createdMs int64
}

// builder packs records into a Snapshot. It is single-goroutine: the index builds snapshots in its own loop.
type builder struct {
	dirIndex map[string]uint32
	dirs     []string
	extIndex map[string]uint16
	exts     []string
	names    []byte
	entries  []entry
	// lastDir caches the most recent directory lookup: scans add many files per directory in a row.
	lastDir   string
	lastDirID uint32
	haveLast  bool
}

// newBuilder returns an empty builder sized for about capacity entries.
func newBuilder(capacity int) *builder {
	return &builder{
		dirIndex: map[string]uint32{},
		extIndex: map[string]uint16{},
		entries:  make([]entry, 0, capacity),
	}
}

// internDir returns the table index of dir, adding it on first use.
func (b *builder) internDir(dir string) uint32 {
	if b.haveLast && dir == b.lastDir {
		return b.lastDirID
	}
	id, ok := b.dirIndex[dir]
	if !ok {
		id = uint32(len(b.dirs))
		b.dirs = append(b.dirs, dir)
		b.dirIndex[dir] = id
	}
	b.lastDir, b.lastDirID, b.haveLast = dir, id, true
	return id
}

// internExt returns the table index of ext, adding it on first use. It reports false when the table is full.
func (b *builder) internExt(ext string) (uint16, bool) {
	if id, ok := b.extIndex[ext]; ok {
		return id, true
	}
	if len(b.exts) >= maxExtensions {
		return 0, false
	}
	id := uint16(len(b.exts))
	b.exts = append(b.exts, ext)
	b.extIndex[ext] = id
	return id, true
}

// appendName copies name into the arena and returns its offset. It reports false when the arena would exceed
// the 32-bit offsets entries use.
func (b *builder) appendName(name []byte) (uint32, bool) {
	if len(b.names)+len(name) > math.MaxUint32 {
		return 0, false
	}
	offset := uint32(len(b.names))
	b.names = append(b.names, name...)
	return offset, true
}

// addRecord packs one file. It reports false, and adds nothing, when the name cannot be represented.
func (b *builder) addRecord(r record) bool {
	if r.name == "" || len(r.name) > maxNameBytes {
		return false
	}
	ext := ExtensionOf(r.name)
	extID, ok := b.internExt(ext)
	if !ok {
		return false
	}
	offset, ok := b.appendName([]byte(r.name))
	if !ok {
		return false
	}
	b.entries = append(b.entries, entry{
		dir:       b.internDir(r.dir),
		nameOff:   offset,
		nameLen:   uint16(len(r.name)),
		ext:       extID,
		kind:      kindOfExtension(ext),
		size:      r.size,
		modMs:     r.modMs,
		createdMs: r.createdMs,
	})
	return true
}

// appendFrom copies one entry of old into the builder, remapping its directory and extension through the
// caller's tables (-1 means not yet mapped). It reports false when the copy cannot be represented.
func (b *builder) appendFrom(old *Snapshot, e entry, dirMap, extMap []int32) bool {
	if dirMap[e.dir] < 0 {
		dirMap[e.dir] = int32(b.internDir(old.dirs[e.dir]))
	}
	if extMap[e.ext] < 0 {
		id, ok := b.internExt(old.exts[e.ext])
		if !ok {
			return false
		}
		extMap[e.ext] = int32(id)
	}
	offset, ok := b.appendName(old.nameOf(e))
	if !ok {
		return false
	}
	e.dir = uint32(dirMap[e.dir])
	e.ext = uint16(extMap[e.ext])
	e.nameOff = offset
	b.entries = append(b.entries, e)
	return true
}

// build finishes the snapshot. The builder must not be used afterwards.
func (b *builder) build(generation uint64, configKey string) *Snapshot {
	return &Snapshot{
		generation: generation,
		configKey:  configKey,
		dirs:       b.dirs,
		exts:       b.exts,
		names:      b.names,
		entries:    b.entries,
	}
}

// emptySnapshot returns a snapshot with no files, the state before anything has been scanned or loaded.
func emptySnapshot(configKey string) *Snapshot {
	return newBuilder(0).build(0, configKey)
}
