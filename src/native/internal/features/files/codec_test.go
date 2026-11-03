package files

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"slices"
	"testing"
)

// encodedFixture builds a small snapshot and returns its encoding under configKey "key".
func encodedFixture(t testing.TB) (*Snapshot, []byte) {
	t.Helper()
	b := newBuilder(4)
	b.addRecord(record{dir: "/home/me/Documents", name: "a.pdf", size: 10, modMs: 1000, createdMs: 900})
	b.addRecord(record{dir: "/home/me/Documents/sub", name: "photo.HEIC", size: 1 << 33, modMs: 2000, createdMs: 2000})
	snap := b.build(3, "key")
	return snap, encodeSnapshot(snap)
}

// resign recomputes the trailing checksum after a test edited the body, so the edit reaches the field checks.
func resign(data []byte) []byte {
	if len(data) < 4 {
		return data
	}
	body := append([]byte(nil), data[:len(data)-4]...)
	return binary.LittleEndian.AppendUint32(body, crc32.ChecksumIEEE(body))
}

// TestSnapshotRoundTripKeepsEveryField encodes and decodes a snapshot and compares what a reader would see.
func TestSnapshotRoundTripKeepsEveryField(t *testing.T) {
	snap, data := encodedFixture(t)
	got, err := decodeSnapshot(data, "key")
	if err != nil {
		t.Fatalf("decodeSnapshot: %v", err)
	}
	if got.Len() != snap.Len() {
		t.Fatalf("decoded %d files, want %d", got.Len(), snap.Len())
	}
	for i := range snap.entries {
		if snap.Path(i) != got.Path(i) || snap.Info(i) != got.Info(i) {
			t.Fatalf("entry %d differs: %q %+v vs %q %+v", i, snap.Path(i), snap.Info(i), got.Path(i), got.Info(i))
		}
	}
	if got.Info(1).Size != 1<<33 {
		t.Fatal("sizes above 4 GiB must survive the round trip")
	}
}

// TestDecodeRejectsEveryTruncation cuts an encoding at every length; each cut must be refused without a panic.
func TestDecodeRejectsEveryTruncation(t *testing.T) {
	_, data := encodedFixture(t)
	for n := range len(data) {
		if _, err := decodeSnapshot(data[:n], "key"); err == nil {
			t.Fatalf("a snapshot cut to %d of %d bytes decoded", n, len(data))
		}
	}
}

// TestDecodeRefusesAFlippedByte checks the checksum catches corruption anywhere in the body.
func TestDecodeRefusesAFlippedByte(t *testing.T) {
	_, data := encodedFixture(t)
	for _, at := range []int{0, 9, len(data) / 2, len(data) - 5} {
		broken := slices.Clone(data)
		broken[at] ^= 0xff
		if _, err := decodeSnapshot(broken, "key"); !errors.Is(err, ErrSnapshotCorrupt) {
			t.Fatalf("flipped byte %d: err = %v, want ErrSnapshotCorrupt", at, err)
		}
	}
}

// TestDecodeRefusesAnotherConfiguration checks that a snapshot built for other roots or rules is not served.
func TestDecodeRefusesAnotherConfiguration(t *testing.T) {
	_, data := encodedFixture(t)
	if _, err := decodeSnapshot(data, "other"); !errors.Is(err, ErrSnapshotConfig) {
		t.Fatalf("err = %v, want ErrSnapshotConfig", err)
	}
}

// TestDecodeRefusesAnotherVersion checks that a future or past layout is ignored, not misread.
func TestDecodeRefusesAnotherVersion(t *testing.T) {
	_, data := encodedFixture(t)
	edited := slices.Clone(data)
	binary.LittleEndian.PutUint32(edited[len(codecMagic):], codecVersion+1)
	if _, err := decodeSnapshot(resign(edited), "key"); !errors.Is(err, ErrSnapshotVersion) {
		t.Fatalf("err = %v, want ErrSnapshotVersion", err)
	}
}

// TestDecodeRejectsOutOfRangeIndexesEvenWithAValidChecksum edits an entry so it points past the tables; the checksum is
// recomputed so only the bounds checks can refuse it.
func TestDecodeRejectsOutOfRangeIndexesEvenWithAValidChecksum(t *testing.T) {
	snap, data := encodedFixture(t)
	entriesStart := len(data) - 4 - len(snap.entries)*entrySize
	bad := slices.Clone(data)
	binary.LittleEndian.PutUint32(bad[entriesStart:], 999)
	if _, err := decodeSnapshot(resign(bad), "key"); !errors.Is(err, ErrSnapshotCorrupt) {
		t.Fatalf("dir index past the table: err = %v, want ErrSnapshotCorrupt", err)
	}
	badKind := slices.Clone(data)
	badKind[entriesStart+12] = 77
	if _, err := decodeSnapshot(resign(badKind), "key"); !errors.Is(err, ErrSnapshotCorrupt) {
		t.Fatalf("unknown kind: err = %v, want ErrSnapshotCorrupt", err)
	}
}

// FuzzDecodeSnapshot checks that no byte sequence makes the decoder panic.
func FuzzDecodeSnapshot(f *testing.F) {
	_, data := encodedFixture(f)
	f.Add(data)
	f.Add(resign(data[:len(data)/2]))
	f.Add([]byte("NVMFIDX1"))
	f.Fuzz(func(t *testing.T, input []byte) {
		_, _ = decodeSnapshot(input, "key")
		_, _ = decodeSnapshot(resign(input), "key")
	})
}
