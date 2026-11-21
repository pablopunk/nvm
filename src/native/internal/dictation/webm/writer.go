// Package webm writes the one container the transcription route accepts: a WebM file with a single Opus track.
// A dictation segment is small enough to hold in memory, so the whole file is built at once. Layer: dictation.
package webm

import (
	"encoding/binary"
	"math"
)

// Track describes the Opus track: the input sample rate and channel count the packets were encoded from, and the
// encoder's lookahead in samples at that rate, which the OpusHead records as pre-skip.
type Track struct {
	SampleRate int
	Channels   int
	Lookahead  int
}

// clusterMillis is how much audio one Matroska cluster holds. One-second clusters keep timecodes small and let a
// reader seek by second.
const clusterMillis = 1000

// opusHeadRate is the rate OpusHead pre-skip is counted in: Opus always decodes at 48 kHz.
const opusHeadRate = 48000

// Element IDs of the EBML and Matroska elements the writer emits.
var (
	idEBML            = []byte{0x1A, 0x45, 0xDF, 0xA3}
	idEBMLVersion     = []byte{0x42, 0x86}
	idEBMLReadVersion = []byte{0x42, 0xF7}
	idEBMLMaxIDLength = []byte{0x42, 0xF2}
	idEBMLMaxSize     = []byte{0x42, 0xF3}
	idDocType         = []byte{0x42, 0x82}
	idDocTypeVersion  = []byte{0x42, 0x87}
	idDocTypeRead     = []byte{0x42, 0x85}
	idSegment         = []byte{0x18, 0x53, 0x80, 0x67}
	idInfo            = []byte{0x15, 0x49, 0xA9, 0x66}
	idTimecodeScale   = []byte{0x2A, 0xD7, 0xB1}
	idDuration        = []byte{0x44, 0x89}
	idMuxingApp       = []byte{0x4D, 0x80}
	idWritingApp      = []byte{0x57, 0x41}
	idTracks          = []byte{0x16, 0x54, 0xAE, 0x6B}
	idTrackEntry      = []byte{0xAE}
	idTrackNumber     = []byte{0xD7}
	idTrackUID        = []byte{0x73, 0xC5}
	idTrackType       = []byte{0x83}
	idFlagLacing      = []byte{0x9C}
	idCodecID         = []byte{0x86}
	idCodecPrivate    = []byte{0x63, 0xA2}
	idAudio           = []byte{0xE1}
	idSamplingRate    = []byte{0xB5}
	idChannels        = []byte{0x9F}
	idCluster         = []byte{0x1F, 0x43, 0xB6, 0x75}
	idTimecode        = []byte{0xE7}
	idSimpleBlock     = []byte{0xA3}
)

// Mux returns the WebM file for packets, one Opus packet per frameMillis of audio, in order. A track with no
// channels or a non-positive frame length is a caller bug and yields an empty file.
func Mux(track Track, packets [][]byte, frameMillis int) []byte {
	if track.Channels <= 0 || track.SampleRate <= 0 || frameMillis <= 0 {
		return nil
	}
	segment := concat(
		infoElement(float64(len(packets)*frameMillis)),
		tracksElement(track),
		clustersElement(packets, frameMillis),
	)
	return concat(headerElement(), element(idSegment, segment))
}

func headerElement() []byte {
	return element(idEBML, concat(
		uintElement(idEBMLVersion, 1),
		uintElement(idEBMLReadVersion, 1),
		uintElement(idEBMLMaxIDLength, 4),
		uintElement(idEBMLMaxSize, 8),
		stringElement(idDocType, "webm"),
		uintElement(idDocTypeVersion, 4),
		uintElement(idDocTypeRead, 2),
	))
}

func infoElement(durationMillis float64) []byte {
	return element(idInfo, concat(
		uintElement(idTimecodeScale, 1_000_000),
		floatElement(idDuration, durationMillis),
		stringElement(idMuxingApp, "nvm"),
		stringElement(idWritingApp, "nvm"),
	))
}

func tracksElement(track Track) []byte {
	audio := element(idAudio, concat(
		floatElement(idSamplingRate, float64(track.SampleRate)),
		uintElement(idChannels, uint64(track.Channels)),
	))
	return element(idTracks, element(idTrackEntry, concat(
		uintElement(idTrackNumber, 1),
		uintElement(idTrackUID, 1),
		uintElement(idTrackType, 2),
		uintElement(idFlagLacing, 0),
		stringElement(idCodecID, "A_OPUS"),
		element(idCodecPrivate, opusHead(track)),
		audio,
	)))
}

// opusHead is the 19-byte OpusHead packet of RFC 7845: version 1, the channel count, pre-skip at 48 kHz, the input
// rate, and no output gain and the default channel mapping.
func opusHead(track Track) []byte {
	head := make([]byte, 19)
	copy(head, "OpusHead")
	head[8] = 1
	head[9] = byte(track.Channels)
	binary.LittleEndian.PutUint16(head[10:], uint16(track.Lookahead*opusHeadRate/track.SampleRate))
	binary.LittleEndian.PutUint32(head[12:], uint32(track.SampleRate))
	return head
}

// clustersElement writes the packets as SimpleBlocks, grouped into one-second clusters. Every block is a keyframe,
// as every Opus packet decodes on its own.
func clustersElement(packets [][]byte, frameMillis int) []byte {
	var out []byte
	perCluster := max(1, clusterMillis/frameMillis)
	for start := 0; start < len(packets); start += perCluster {
		end := min(start+perCluster, len(packets))
		clusterTime := start * frameMillis
		var blocks []byte
		for index := start; index < end; index++ {
			blocks = append(blocks, simpleBlock(index*frameMillis-clusterTime, packets[index])...)
		}
		out = append(out, element(idCluster, concat(uintElement(idTimecode, uint64(clusterTime)), blocks))...)
	}
	return out
}

// simpleBlock is one packet on track 1: the track number as a one-byte EBML vint, the timecode relative to its
// cluster as a signed 16-bit value, the keyframe flag, and the packet.
func simpleBlock(relativeMillis int, packet []byte) []byte {
	body := make([]byte, 4, 4+len(packet))
	body[0] = 0x81
	binary.BigEndian.PutUint16(body[1:], uint16(int16(relativeMillis)))
	body[3] = 0x80
	return element(idSimpleBlock, append(body, packet...))
}

// element is an ID, an 8-byte EBML size and the payload. Eight bytes always encode the size, so the writer never
// needs the size of a payload before it writes the payload.
func element(id []byte, payload []byte) []byte {
	out := make([]byte, 0, len(id)+8+len(payload))
	out = append(out, id...)
	out = append(out, eightByteSize(len(payload))...)
	return append(out, payload...)
}

func eightByteSize(n int) []byte {
	size := make([]byte, 8)
	binary.BigEndian.PutUint64(size, uint64(n))
	size[0] = 0x01
	return size
}

// uintElement stores v in the fewest big-endian bytes, as EBML unsigned integers are stored.
func uintElement(id []byte, v uint64) []byte {
	var raw []byte
	for shift := 56; shift >= 0; shift -= 8 {
		if len(raw) == 0 && v>>uint(shift) == 0 && shift > 0 {
			continue
		}
		raw = append(raw, byte(v>>uint(shift)))
	}
	return element(id, raw)
}

func floatElement(id []byte, v float64) []byte {
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, math.Float64bits(v))
	return element(id, raw)
}

func stringElement(id []byte, s string) []byte {
	return element(id, []byte(s))
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}
