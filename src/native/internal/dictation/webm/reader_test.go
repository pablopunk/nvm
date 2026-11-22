package webm

import (
	"bytes"
	"testing"
)

func TestDemuxReadsBackWhatMuxWrote(t *testing.T) {
	track := Track{SampleRate: 16000, Channels: 1, Lookahead: 312}
	packets := make([][]byte, 130)
	for i := range packets {
		packets[i] = []byte{byte(i), byte(i >> 8), 0xAB}
	}
	gotTrack, gotPackets, err := Demux(Mux(track, packets, 20))
	if err != nil {
		t.Fatalf("Demux: %v", err)
	}
	if gotTrack != track {
		t.Fatalf("track = %+v, want %+v", gotTrack, track)
	}
	if len(gotPackets) != len(packets) {
		t.Fatalf("got %d packets, want %d", len(gotPackets), len(packets))
	}
	for i := range packets {
		if !bytes.Equal(gotPackets[i], packets[i]) {
			t.Fatalf("packet %d = %v, want %v", i, gotPackets[i], packets[i])
		}
	}
}

func TestDemuxRefusesATruncatedFile(t *testing.T) {
	file := Mux(Track{SampleRate: 16000, Channels: 1}, [][]byte{{1, 2, 3}}, 20)
	if _, _, err := Demux(file[:len(file)-2]); err == nil {
		t.Fatal("a truncated file was read")
	}
	if _, _, err := Demux([]byte("not webm")); err == nil {
		t.Fatal("a file that is not WebM was read")
	}
}
