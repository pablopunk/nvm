package webm

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// rawElement is one parsed EBML element: its ID bytes and payload.
type rawElement struct {
	id      []byte
	payload []byte
}

// parseElements reads consecutive elements from data, with one-to-eight-byte IDs and sizes, as the EBML spec sets them.
func parseElements(t *testing.T, data []byte) []rawElement {
	t.Helper()
	var out []rawElement
	for len(data) > 0 {
		idLen := vintLength(t, data[0])
		id := data[:idLen]
		data = data[idLen:]
		sizeLen := vintLength(t, data[0])
		size := vintValue(data[:sizeLen])
		data = data[sizeLen:]
		if uint64(len(data)) < size {
			t.Fatalf("element %x claims %d bytes, only %d remain", id, size, len(data))
		}
		out = append(out, rawElement{id: id, payload: data[:size]})
		data = data[size:]
	}
	return out
}

func vintLength(t *testing.T, first byte) int {
	t.Helper()
	for length := 1; length <= 8; length++ {
		if first&(0x80>>(length-1)) != 0 {
			return length
		}
	}
	t.Fatalf("byte %x starts no EBML vint", first)
	return 0
}

func vintValue(raw []byte) uint64 {
	value := uint64(raw[0] & (0xFF >> len(raw)))
	for _, b := range raw[1:] {
		value = value<<8 | uint64(b)
	}
	return value
}

func uintPayload(raw []byte) uint64 {
	var value uint64
	for _, b := range raw {
		value = value<<8 | uint64(b)
	}
	return value
}

func find(elements []rawElement, id []byte) (rawElement, bool) {
	for _, element := range elements {
		if bytes.Equal(element.id, id) {
			return element, true
		}
	}
	return rawElement{}, false
}

func TestMuxWritesAnEBMLHeaderAndOneOpusTrack(t *testing.T) {
	packets := [][]byte{{1, 2, 3}, {4, 5}}
	file := Mux(Track{SampleRate: 16000, Channels: 1, Lookahead: 312}, packets, 20)
	top := parseElements(t, file)
	header, ok := find(top, idEBML)
	if !ok {
		t.Fatalf("no EBML header")
	}
	if doc, _ := find(parseElements(t, header.payload), idDocType); string(doc.payload) != "webm" {
		t.Fatalf("DocType = %q, want webm", doc.payload)
	}
	segment, ok := find(top, idSegment)
	if !ok {
		t.Fatalf("no Segment")
	}
	tracks, ok := find(parseElements(t, segment.payload), idTracks)
	if !ok {
		t.Fatalf("no Tracks")
	}
	entry, ok := find(parseElements(t, tracks.payload), idTrackEntry)
	if !ok {
		t.Fatalf("no TrackEntry")
	}
	fields := parseElements(t, entry.payload)
	if codec, _ := find(fields, idCodecID); string(codec.payload) != "A_OPUS" {
		t.Fatalf("CodecID = %q, want A_OPUS", codec.payload)
	}
	private, _ := find(fields, idCodecPrivate)
	if string(private.payload[:8]) != "OpusHead" || len(private.payload) != 19 || private.payload[8] != 1 || private.payload[9] != 1 {
		t.Fatalf("CodecPrivate = %x, want an OpusHead of version 1 and one channel", private.payload)
	}
	if preSkip := binary.LittleEndian.Uint16(private.payload[10:]); preSkip != 936 {
		t.Fatalf("pre-skip = %d, want 312 samples at 16 kHz scaled to 48 kHz (936)", preSkip)
	}
	if rate := binary.LittleEndian.Uint32(private.payload[12:]); rate != 16000 {
		t.Fatalf("input rate = %d, want 16000", rate)
	}
}

func TestMuxKeepsEveryPacketInOrderWithItsTimecode(t *testing.T) {
	packets := make([][]byte, 120)
	for i := range packets {
		packets[i] = []byte{byte(i), byte(i >> 8), 0xAA}
	}
	file := Mux(Track{SampleRate: 16000, Channels: 1}, packets, 20)
	segment, _ := find(parseElements(t, file), idSegment)
	var blocks [][]byte
	var clusterTimes []uint64
	for _, element := range parseElements(t, segment.payload) {
		if !bytes.Equal(element.id, idCluster) {
			continue
		}
		cluster := parseElements(t, element.payload)
		timecode, _ := find(cluster, idTimecode)
		clusterTimes = append(clusterTimes, uintPayload(timecode.payload))
		for _, child := range cluster {
			if !bytes.Equal(child.id, idSimpleBlock) {
				continue
			}
			relative := int16(binary.BigEndian.Uint16(child.payload[1:3]))
			if child.payload[3] != 0x80 {
				t.Fatalf("block flags = %x, want keyframe", child.payload[3])
			}
			if got := int(relative); got != (len(blocks)%50)*20 {
				t.Fatalf("block %d relative timecode = %d, want %d", len(blocks), got, (len(blocks)%50)*20)
			}
			blocks = append(blocks, child.payload[4:])
		}
	}
	if len(blocks) != len(packets) {
		t.Fatalf("got %d blocks, want %d", len(blocks), len(packets))
	}
	for i := range packets {
		if !bytes.Equal(blocks[i], packets[i]) {
			t.Fatalf("block %d = %x, want %x", i, blocks[i], packets[i])
		}
	}
	if want := []uint64{0, 1000, 2000}; !equalUint64s(clusterTimes, want) {
		t.Fatalf("cluster timecodes = %v, want %v: one-second clusters", clusterTimes, want)
	}
}

func TestMuxRefusesAnUnusableTrack(t *testing.T) {
	if got := Mux(Track{SampleRate: 16000}, [][]byte{{1}}, 20); got != nil {
		t.Fatalf("zero channels produced %d bytes", len(got))
	}
	if got := Mux(Track{SampleRate: 16000, Channels: 1}, [][]byte{{1}}, 0); got != nil {
		t.Fatalf("zero frame length produced %d bytes", len(got))
	}
}

func equalUint64s(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
