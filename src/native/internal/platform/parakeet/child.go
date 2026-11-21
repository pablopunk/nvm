package parakeet

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ChildFlag is the first argument that makes the binary load the model and transcribe what its parent sends until
// its input closes. The parent starts it when a dictation starts and ends it once dictation no longer needs it, so
// the model's memory lives only as long as the child.
const ChildFlag = "--nvm-speech-child"

// maxRequestSamples bounds one request: two minutes of 16 kHz audio, beyond the longest segment a dictation sends.
const maxRequestSamples = 2 * 60 * 16000

// Reply is one line the child writes: Ready once the model is loaded, then Text or Error for each request in order.
// An Error before Ready means the model could not be loaded, and the child exits.
type Reply struct {
	Ready bool   `json:"ready,omitempty"`
	Text  string `json:"text"`
	Error string `json:"error,omitempty"`
}

// WriteRequest sends 16 kHz mono samples to a child: their count as a little-endian uint32, then the samples.
func WriteRequest(w io.Writer, pcm []int16) error {
	header := make([]byte, 4, 4+2*len(pcm))
	binary.LittleEndian.PutUint32(header, uint32(len(pcm)))
	for _, sample := range pcm {
		header = binary.LittleEndian.AppendUint16(header, uint16(sample))
	}
	_, err := w.Write(header)
	return err
}

// Main runs the child when args ask for it. handled is false for any other invocation of the binary.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int, handled bool) {
	if len(args) < 2 || args[1] != ChildFlag {
		return 0, false
	}
	if len(args) != 4 {
		fmt.Fprintln(stderr, "speech recognition was started with the wrong arguments")
		return 2, true
	}
	encoder := json.NewEncoder(stdout)
	model, err := Load(args[2], args[3])
	if err != nil {
		_ = encoder.Encode(Reply{Error: err.Error()})
		return 1, true
	}
	defer model.Close()
	if err := encoder.Encode(Reply{Ready: true}); err != nil {
		return 1, true
	}
	if err := serve(bufio.NewReader(stdin), encoder, model); err != nil {
		fmt.Fprintln(stderr, err)
		return 1, true
	}
	return 0, true
}

// serve answers requests until the input ends.
func serve(in io.Reader, out *json.Encoder, model *Model) error {
	header := make([]byte, 4)
	for {
		if _, err := io.ReadFull(in, header); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		count := binary.LittleEndian.Uint32(header)
		if count > maxRequestSamples {
			return fmt.Errorf("a speech request of %d samples is too long", count)
		}
		raw := make([]byte, 2*count)
		if _, err := io.ReadFull(in, raw); err != nil {
			return err
		}
		samples := make([]float32, count)
		for i := range samples {
			samples[i] = float32(int16(binary.LittleEndian.Uint16(raw[2*i:]))) / 32768
		}
		text, err := model.Transcribe(context.Background(), samples)
		reply := Reply{Text: text}
		if err != nil {
			reply.Error = err.Error()
		}
		if err := out.Encode(reply); err != nil {
			return err
		}
	}
}
