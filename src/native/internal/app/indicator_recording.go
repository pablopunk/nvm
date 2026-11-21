package app

// appendRecordingSample never carries a trace between microphone operations.
func (i *aiIndicator) appendRecordingSample(operation string, level float64) {
	if i.recordingOperation != operation {
		clear(i.status.Waveform[:])
		i.recordingOperation = operation
	}
	copy(i.status.Waveform[:], i.status.Waveform[1:])
	i.status.Waveform[len(i.status.Waveform)-1] = level
}
