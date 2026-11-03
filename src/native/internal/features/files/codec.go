package files

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// codecVersion is the on-disk version of an encoded snapshot. Bump it when the layout changes; a snapshot with another
// version is ignored and the index scans again.
const codecVersion = 1

// codecMagic opens every encoded snapshot.
var codecMagic = []byte("NVMFIDX1")

// Errors returned when an encoded snapshot cannot be used. The index treats all of them the same way: it logs and
// scans again.
var (
	ErrSnapshotCorrupt  = errors.New("files: stored index snapshot is corrupt")
	ErrSnapshotVersion  = errors.New("files: stored index snapshot has an unsupported version")
	ErrSnapshotConfig   = errors.New("files: stored index snapshot was built for another configuration")
	errSnapshotTruncate = errors.New("files: stored index snapshot ends early")
)

// encodedEntrySize is the on-disk size of one entry: the in-memory entry layout with explicit little-endian fields.
const encodedEntrySize = entrySize

// encodeSnapshot serializes a snapshot. The layout is: magic, version, configuration key, the directory table, the
// extension table, the name arena, the entries and a CRC-32 of everything before it.
func encodeSnapshot(s *Snapshot) []byte {
	size := len(codecMagic) + 4 + 4 + len(s.configKey) + 4 + 8 + len(s.names) + 8 + len(s.entries)*encodedEntrySize + 4
	for _, d := range s.dirs {
		size += 4 + len(d)
	}
	for _, x := range s.exts {
		size += 4 + len(x)
	}
	out := make([]byte, 0, size)
	out = append(out, codecMagic...)
	out = binary.LittleEndian.AppendUint32(out, codecVersion)
	out = appendString(out, s.configKey)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(s.dirs)))
	for _, d := range s.dirs {
		out = appendString(out, d)
	}
	out = binary.LittleEndian.AppendUint32(out, uint32(len(s.exts)))
	for _, x := range s.exts {
		out = appendString(out, x)
	}
	out = binary.LittleEndian.AppendUint64(out, uint64(len(s.names)))
	out = append(out, s.names...)
	out = binary.LittleEndian.AppendUint64(out, uint64(len(s.entries)))
	for _, e := range s.entries {
		out = appendEntry(out, e)
	}
	return binary.LittleEndian.AppendUint32(out, crc32.ChecksumIEEE(out))
}

// appendString writes a 32-bit length and the bytes of s.
func appendString(out []byte, s string) []byte {
	out = binary.LittleEndian.AppendUint32(out, uint32(len(s)))
	return append(out, s...)
}

// appendEntry writes one entry in the fixed 40-byte layout.
func appendEntry(out []byte, e entry) []byte {
	out = binary.LittleEndian.AppendUint32(out, e.dir)
	out = binary.LittleEndian.AppendUint32(out, e.nameOff)
	out = binary.LittleEndian.AppendUint16(out, e.nameLen)
	out = binary.LittleEndian.AppendUint16(out, e.ext)
	out = append(out, byte(e.kind), 0, 0, 0)
	out = binary.LittleEndian.AppendUint64(out, uint64(e.size))
	out = binary.LittleEndian.AppendUint64(out, uint64(e.modMs))
	return binary.LittleEndian.AppendUint64(out, uint64(e.createdMs))
}

// decodeSnapshot parses an encoded snapshot built for configKey. It never panics on malformed input: every length
// and offset is checked against the bytes that exist before it is used, and the checksum is verified first.
func decodeSnapshot(data []byte, configKey string) (*Snapshot, error) {
	if len(data) < len(codecMagic)+4+4 {
		return nil, ErrSnapshotCorrupt
	}
	body, sum := data[:len(data)-4], data[len(data)-4:]
	if crc32.ChecksumIEEE(body) != binary.LittleEndian.Uint32(sum) {
		return nil, ErrSnapshotCorrupt
	}
	if !bytes.Equal(body[:len(codecMagic)], codecMagic) {
		return nil, ErrSnapshotCorrupt
	}
	d := &decoder{data: body, off: len(codecMagic)}
	if version := d.uint32(); d.err == nil && version != codecVersion {
		return nil, ErrSnapshotVersion
	}
	storedKey := d.readString()
	if d.err != nil {
		return nil, ErrSnapshotCorrupt
	}
	if storedKey != configKey {
		return nil, ErrSnapshotConfig
	}
	dirs := d.readStrings()
	exts := d.readStrings()
	names := d.readBlob()
	entries := d.entries(len(dirs), len(exts), len(names))
	if d.err != nil || d.off != len(body) {
		return nil, ErrSnapshotCorrupt
	}
	return &Snapshot{configKey: storedKey, dirs: dirs, exts: exts, names: names, entries: entries}, nil
}

// decoder reads the fields of a snapshot body. The first failure is sticky in err.
type decoder struct {
	data []byte
	off  int
	err  error
}

// take returns the next n bytes, or records a truncation.
func (d *decoder) take(n uint64) []byte {
	if d.err != nil {
		return nil
	}
	if n > uint64(len(d.data)-d.off) {
		d.err = errSnapshotTruncate
		return nil
	}
	out := d.data[d.off : d.off+int(n)]
	d.off += int(n)
	return out
}

// uint32 reads a little-endian 32-bit value.
func (d *decoder) uint32() uint32 {
	b := d.take(4)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}

// uint64 reads a little-endian 64-bit value.
func (d *decoder) uint64() uint64 {
	b := d.take(8)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}

// readString reads a length-prefixed string.
func (d *decoder) readString() string {
	n := d.uint32()
	return string(d.take(uint64(n)))
}

// readBlob reads a 64-bit length and that many bytes, copied out of the input.
func (d *decoder) readBlob() []byte {
	n := d.uint64()
	b := d.take(n)
	if d.err != nil {
		return nil
	}
	return append([]byte(nil), b...)
}

// readStrings reads a counted list of strings. The count is bounded by the bytes left, since each string takes at
// least four bytes, so a corrupt count cannot force a huge allocation.
func (d *decoder) readStrings() []string {
	n := d.uint32()
	if d.err != nil || uint64(n) > uint64(len(d.data)-d.off)/4 {
		d.err = errSnapshotTruncate
		return nil
	}
	out := make([]string, 0, n)
	for range n {
		out = append(out, d.readString())
		if d.err != nil {
			return nil
		}
	}
	return out
}

// entries reads the counted entry list and checks every index against the tables it refers to.
func (d *decoder) entries(numDirs, numExts, nameBytes int) []entry {
	n := d.uint64()
	if d.err != nil || n > uint64(len(d.data)-d.off)/encodedEntrySize {
		d.err = errSnapshotTruncate
		return nil
	}
	out := make([]entry, 0, n)
	for range n {
		raw := d.take(encodedEntrySize)
		if d.err != nil {
			return nil
		}
		e := entry{
			dir:       binary.LittleEndian.Uint32(raw[0:4]),
			nameOff:   binary.LittleEndian.Uint32(raw[4:8]),
			nameLen:   binary.LittleEndian.Uint16(raw[8:10]),
			ext:       binary.LittleEndian.Uint16(raw[10:12]),
			kind:      Kind(raw[12]),
			size:      int64(binary.LittleEndian.Uint64(raw[16:24])),
			modMs:     int64(binary.LittleEndian.Uint64(raw[24:32])),
			createdMs: int64(binary.LittleEndian.Uint64(raw[32:40])),
		}
		if int(e.dir) >= numDirs || int(e.ext) >= numExts || int(e.kind) > int(KindVideo) ||
			uint64(e.nameOff)+uint64(e.nameLen) > uint64(nameBytes) {
			d.err = ErrSnapshotCorrupt
			return nil
		}
		out = append(out, e)
	}
	return out
}
