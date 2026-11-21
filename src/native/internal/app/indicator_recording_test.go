package app

import "testing"

func TestRecordingTraceBelongsToOneOperation(t *testing.T) {
	i := &aiIndicator{}
	i.appendRecordingSample("first", 0.8)
	i.appendRecordingSample("first", 0.4)
	n := len(i.status.Waveform)
	if i.status.Waveform[n-2] != 0.8 {
		t.Fatal("same recording lost its trace")
	}
	i.appendRecordingSample("second", 0.2)
	for _, level := range i.status.Waveform[:n-1] {
		if level != 0 {
			t.Fatal("new recording inherited a trace")
		}
	}
	if i.status.Waveform[n-1] != 0.2 {
		t.Fatal("new recording lost its sample")
	}
}
