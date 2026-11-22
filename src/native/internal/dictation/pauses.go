package dictation

import "math"

// Live segmenting. A segment ends at a natural pause once it holds enough speech to be worth a request, so it can be
// transcribed while the user goes on speaking and only the words after the last pause are left when they stop. The
// minimum keeps a minute of speech near a dozen requests, well inside the backend's per-minute limit; past the long
// mark a shorter pause is enough, so fast talkers still get cut. A minute without any pause is cut as before.
const (
	minLiveSegmentMillis  = 5_000
	longLiveSegmentMillis = 12_000
	pauseMillis           = 500
	longPauseMillis       = 250
)

// A frame is quiet when it sits within floorMarginDB of the noise floor or voiceDropDB below the recent voice. The floor
// falls to any quieter frame at once and rises slowly; the voice rises to any louder frame at once and decays slowly.
// Measuring against the voice as well finds the pauses of a noisy room, whose floor a single quiet frame drags too low.
const (
	floorMarginDB = 6.0
	voiceDropDB   = 20.0
	silenceDB     = -120.0
	floorRiseDB   = 3.0 * FrameMillis / 1000
	voiceDecayDB  = 1.0 * FrameMillis / 1000
)

// pauseDetector decides, frame by frame, where the open segment may end.
type pauseDetector struct {
	floor    float64
	voice    float64
	started  bool
	quietRun int
}

// cut takes the next frame of a segment that already holds frames packets and reports whether the segment should end
// after it.
func (p *pauseDetector) cut(frame []int16, frames int) bool {
	if !p.quiet(frameDB(frame)) {
		p.quietRun = 0
		return false
	}
	p.quietRun++
	millis, quiet := frames*FrameMillis, p.quietRun*FrameMillis
	if millis >= longLiveSegmentMillis && quiet >= longPauseMillis || millis >= minLiveSegmentMillis && quiet >= pauseMillis {
		p.quietRun = 0
		return true
	}
	return false
}

// quiet follows the floor and the voice with one more frame and reports whether that frame is part of a pause.
func (p *pauseDetector) quiet(level float64) bool {
	if !p.started {
		p.floor, p.voice, p.started = level, level, true
	}
	if level < p.floor {
		p.floor = level
	} else {
		p.floor = min(level, p.floor+floorRiseDB)
	}
	if level > p.voice {
		p.voice = level
	} else {
		p.voice -= voiceDecayDB
	}
	return level < p.floor+floorMarginDB || level < p.voice-voiceDropDB
}

// frameDB is the frame's loudness in dBFS; digital silence reads silenceDB.
func frameDB(frame []int16) float64 {
	level := Level(frame)
	if level <= 0 {
		return silenceDB
	}
	return max(silenceDB, 20*math.Log10(level))
}
