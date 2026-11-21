package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteDictation records and transcribes speech on the host. Audio never reaches the extension.
type remoteDictation struct {
	link *hostLink
}

func (d remoteDictation) Status() (api.DictationStatus, error) {
	var status api.DictationStatus
	err := d.link.Call(protocol.ServiceDictation, protocol.OpStatus, nil, &status)
	return status, err
}

func (d remoteDictation) APIAvailable() (bool, error) {
	var available bool
	err := d.link.Call(protocol.ServiceDictation, protocol.OpAPIAvailable, nil, &available)
	return available, err
}

func (d remoteDictation) Devices() ([]api.DictationDevice, error) {
	var devices []api.DictationDevice
	err := d.link.Call(protocol.ServiceDictation, protocol.OpDevices, nil, &devices)
	return devices, err
}

func (d remoteDictation) Start(options api.DictationStartOptions) error {
	return d.link.Call(protocol.ServiceDictation, protocol.OpStart, options, nil)
}

func (d remoteDictation) Stop() (string, error) {
	var transcript string
	err := d.link.Call(protocol.ServiceDictation, protocol.OpStop, nil, &transcript)
	return transcript, err
}

func (d remoteDictation) Cancel() error {
	return d.link.Call(protocol.ServiceDictation, protocol.OpCancel, nil, nil)
}

func (d remoteDictation) Recordings() ([]api.SavedDictation, error) {
	var saved []api.SavedDictation
	err := d.link.Call(protocol.ServiceDictation, protocol.OpRecordings, nil, &saved)
	return saved, err
}

func (d remoteDictation) Retry(id string) (string, error) {
	var transcript string
	err := d.link.Call(protocol.ServiceDictation, protocol.OpRetry, protocol.DictationRecordingArgs{ID: id}, &transcript)
	return transcript, err
}

func (d remoteDictation) DeleteRecording(id string) error {
	return d.link.Call(protocol.ServiceDictation, protocol.OpDeleteRecording, protocol.DictationRecordingArgs{ID: id}, nil)
}
