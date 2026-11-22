package webm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// errMalformed is a file this package cannot read back: one it did not write, or one cut short.
var errMalformed = errors.New("the recording is not a readable WebM file")

// Demux reads a file Mux wrote back into its track and its Opus packets, in order. It reads the elements Mux emits
// and skips any others, so it serves the app's own recordings, not WebM files in general.
func Demux(file []byte) (Track, [][]byte, error) {
	var track Track
	var packets [][]byte
	err := walk(file, func(id, payload []byte) error {
		switch {
		case bytes.Equal(id, idSegment), bytes.Equal(id, idTracks), bytes.Equal(id, idTrackEntry),
			bytes.Equal(id, idAudio), bytes.Equal(id, idCluster):
			return errDescend
		case bytes.Equal(id, idCodecPrivate):
			if len(payload) < 19 || string(payload[:8]) != "OpusHead" {
				return errMalformed
			}
			track.Channels = int(payload[9])
			track.SampleRate = int(binary.LittleEndian.Uint32(payload[12:]))
			if track.SampleRate > 0 {
				track.Lookahead = int(binary.LittleEndian.Uint16(payload[10:])) * track.SampleRate / opusHeadRate
			}
		case bytes.Equal(id, idSamplingRate) && track.SampleRate == 0 && len(payload) == 8:
			track.SampleRate = int(math.Float64frombits(binary.BigEndian.Uint64(payload)))
		case bytes.Equal(id, idSimpleBlock):
			if len(payload) < 4 {
				return errMalformed
			}
			packets = append(packets, payload[4:])
		}
		return nil
	})
	if err != nil {
		return Track{}, nil, err
	}
	if track.Channels <= 0 || track.SampleRate <= 0 {
		return Track{}, nil, fmt.Errorf("%w: it names no Opus track", errMalformed)
	}
	return track, packets, nil
}

// errDescend tells walk to read an element's payload as more elements.
var errDescend = errors.New("descend")

// walk calls visit on each element of data, and on the children of every element visit descends into.
func walk(data []byte, visit func(id, payload []byte) error) error {
	for len(data) > 0 {
		idLength := vintLength(data[0])
		if idLength == 0 || idLength > len(data) {
			return errMalformed
		}
		id := data[:idLength]
		data = data[idLength:]
		if len(data) == 0 {
			return errMalformed
		}
		sizeLength := vintLength(data[0])
		if sizeLength == 0 || sizeLength > len(data) {
			return errMalformed
		}
		size := uint64(data[0] & (0xFF >> sizeLength))
		for _, b := range data[1:sizeLength] {
			size = size<<8 | uint64(b)
		}
		data = data[sizeLength:]
		if size > uint64(len(data)) {
			return errMalformed
		}
		payload := data[:size]
		data = data[size:]
		switch err := visit(id, payload); {
		case errors.Is(err, errDescend):
			if err := walk(payload, visit); err != nil {
				return err
			}
		case err != nil:
			return err
		}
	}
	return nil
}

// vintLength is how many bytes an EBML variable-length integer takes, read from its first byte; 0 is invalid.
func vintLength(first byte) int {
	for length := 1; length <= 8; length++ {
		if first&(0x80>>(length-1)) != 0 {
			return length
		}
	}
	return 0
}
