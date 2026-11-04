package fake

import (
	"context"
	"io"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpMicrophoneOpen is the operation name accepted by Microphone.FailNext.
const OpMicrophoneOpen = "microphone.open"

// Microphone is a platform.Microphone that replays scripted PCM. Tests set the devices, queue the samples a
// stream returns, and read back what was opened.
type Microphone struct {
	mu      sync.Mutex
	devices []platform.MicrophoneDevice
	samples []int16
	opened  []string
	streams []*MicrophoneStream
	faults  faultSet
}

var _ platform.Microphone = (*Microphone)(nil)

// NewMicrophone returns a microphone with one default device and no samples.
func NewMicrophone() *Microphone {
	return &Microphone{devices: []platform.MicrophoneDevice{{ID: "default", Title: "Built-in Microphone", Default: true}}}
}

// SetDevices replaces the device list.
func (m *Microphone) SetDevices(devices ...platform.MicrophoneDevice) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.devices = append([]platform.MicrophoneDevice(nil), devices...)
}

// Queue appends samples that the next open stream delivers, in order, then ends.
func (m *Microphone) Queue(samples ...int16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.samples = append(m.samples, samples...)
}

// FailNext makes the next call of op return err.
func (m *Microphone) FailNext(op string, err error) {
	m.faults.failNext(op, err)
}

// Opened lists the device IDs Open was called with, in order.
func (m *Microphone) Opened() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.opened...)
}

// Devices returns the scripted device list.
func (m *Microphone) Devices() ([]platform.MicrophoneDevice, error) {
	if err := m.faults.take("microphone.devices"); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]platform.MicrophoneDevice(nil), m.devices...), nil
}

// Open returns a stream over the queued samples. The queue is shared: samples are handed to the first stream
// that reads them.
func (m *Microphone) Open(ctx context.Context, deviceID string, format platform.AudioFormat) (platform.MicrophoneStream, error) {
	if err := m.faults.take(OpMicrophoneOpen); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opened = append(m.opened, deviceID)
	stream := &MicrophoneStream{mu: &m.mu, samples: &m.samples, done: make(chan struct{})}
	m.streams = append(m.streams, stream)
	go func() {
		select {
		case <-ctx.Done():
			_ = stream.Close()
		case <-stream.done:
		}
	}()
	return stream, nil
}

// MicrophoneStream reads the microphone's queued samples and ends with io.EOF once they run out or it is
// closed.
type MicrophoneStream struct {
	mu      *sync.Mutex
	samples *[]int16
	done    chan struct{}
	closed  bool
}

// Read copies queued samples into pcm. Samples queued before Close are still delivered, so a test that stops a
// recording gets every sample it queued; with none left Read returns io.EOF, which is how a scripted source ends.
func (s *MicrophoneStream) Read(pcm []int16) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(*s.samples) == 0 {
		return 0, io.EOF
	}
	n := copy(pcm, *s.samples)
	*s.samples = (*s.samples)[n:]
	return n, nil
}

// Close ends the stream. It is safe to call more than once.
func (s *MicrophoneStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.done)
	}
	return nil
}
