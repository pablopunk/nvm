// Package parakeet transcribes speech on the device with NVIDIA's Parakeet TDT 0.6B v3 (25 European languages, with
// punctuation) on ONNX Runtime, the runtime the app already ships for text recognition. The model runs in a child
// process of the app (see child.go), so its memory is given back as soon as dictation no longer needs it.
//
// Layer: platform. The package imports the ONNX Runtime binding and the standard library.
package parakeet

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	ort "github.com/benedoc-inc/onnxer/onnxruntime"
)

// runtimeAPIVersion is the ONNX Runtime C API the binding speaks, the same runtime text recognition loads.
const runtimeAPIVersion = 23

// The model's files inside its folder, as the ONNX export names them.
const (
	preprocessorFile = "nemo128.onnx"
	encoderFile      = "encoder-model.int8.onnx"
	decoderFile      = "decoder_joint-model.int8.onnx"
	vocabularyFile   = "vocab.txt"
)

// Files lists the files a model folder must hold.
func Files() []string {
	return []string{preprocessorFile, encoderFile, decoderFile, vocabularyFile}
}

// Decoding geometry of the TDT export: the prediction network keeps two LSTM layers of 640 units, the joint network
// answers the vocabulary's scores followed by the scores of the durations 0..4 frames, and at most ten tokens are
// read from one encoder frame before the decoder moves on.
const (
	stateLayers        = 2
	stateUnits         = 640
	maxTokensPerFrame  = 10
	blankToken         = "<blk>"
	wordStartCharacter = "▁"
)

// Model is a loaded Parakeet model. It is not safe for concurrent use.
type Model struct {
	runtime      *ort.Runtime
	env          *ort.Env
	preprocessor *ort.Session
	encoder      *ort.Session
	decoder      *ort.Session
	vocabulary   []string
	blank        int
}

// Load opens the model in dir on the ONNX Runtime library at library.
func Load(library, dir string) (*Model, error) {
	vocabulary, blank, err := readVocabulary(filepath.Join(dir, vocabularyFile))
	if err != nil {
		return nil, err
	}
	rt, err := ort.NewRuntime(library, runtimeAPIVersion)
	if err != nil {
		return nil, fmt.Errorf("load the speech runtime: %w", err)
	}
	m := &Model{runtime: rt, vocabulary: vocabulary, blank: blank}
	if m.env, err = rt.NewEnv("nevermind-parakeet", ort.LoggingLevelError); err != nil {
		m.Close()
		return nil, err
	}
	threads := max(1, min(8, runtime.NumCPU()))
	open := func(name string, intra int) (*ort.Session, error) {
		session, err := rt.NewSession(m.env, filepath.Join(dir, name), &ort.SessionOptions{IntraOpNumThreads: intra, InterOpNumThreads: 1})
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", name, err)
		}
		return session, nil
	}
	if m.preprocessor, err = open(preprocessorFile, 1); err == nil {
		if m.encoder, err = open(encoderFile, threads); err == nil {
			m.decoder, err = open(decoderFile, 1)
		}
	}
	if err != nil {
		m.Close()
		return nil, err
	}
	return m, nil
}

// Close releases the sessions and the runtime.
func (m *Model) Close() {
	for _, session := range []*ort.Session{m.decoder, m.encoder, m.preprocessor} {
		if session != nil {
			session.Close()
		}
	}
	if m.env != nil {
		m.env.Close()
	}
	if m.runtime != nil {
		m.runtime.Close()
	}
}

// Transcribe returns the text of 16 kHz mono samples in -1..1.
func (m *Model) Transcribe(ctx context.Context, samples []float32) (string, error) {
	if len(samples) == 0 {
		return "", nil
	}
	features, featureCount, err := m.features(ctx, samples)
	if err != nil {
		return "", err
	}
	encoded, dims, frames, err := m.encode(ctx, features, featureCount)
	if err != nil {
		return "", err
	}
	tokens, err := m.decode(ctx, encoded, dims, frames)
	if err != nil {
		return "", err
	}
	return m.detokenize(tokens), nil
}

// features turns the waveform into the model's 128 log-mel bands.
func (m *Model) features(ctx context.Context, samples []float32) (*ort.Value, int64, error) {
	waveform, err := ort.NewTensorValue(m.runtime, samples, []int64{1, int64(len(samples))})
	if err != nil {
		return nil, 0, err
	}
	defer waveform.Close()
	length, err := ort.NewTensorValue(m.runtime, []int64{int64(len(samples))}, []int64{1})
	if err != nil {
		return nil, 0, err
	}
	defer length.Close()
	outputs, err := m.preprocessor.Run(ctx, map[string]*ort.Value{"waveforms": waveform, "waveforms_lens": length})
	if err != nil {
		return nil, 0, fmt.Errorf("compute the speech features: %w", err)
	}
	lengths, _, err := ort.GetTensorData[int64](outputs["features_lens"])
	outputs["features_lens"].Close()
	if err != nil || len(lengths) != 1 {
		outputs["features"].Close()
		return nil, 0, fmt.Errorf("read the feature length: %v", err)
	}
	return outputs["features"], lengths[0], nil
}

// encode runs the encoder and returns its output as dims rows of frames values.
func (m *Model) encode(ctx context.Context, features *ort.Value, count int64) ([]float32, int, int, error) {
	defer features.Close()
	length, err := ort.NewTensorValue(m.runtime, []int64{count}, []int64{1})
	if err != nil {
		return nil, 0, 0, err
	}
	defer length.Close()
	outputs, err := m.encoder.Run(ctx, map[string]*ort.Value{"audio_signal": features, "length": length})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("encode the speech: %w", err)
	}
	for _, value := range outputs {
		defer value.Close()
	}
	encoded, shape, err := ort.GetTensorData[float32](outputs["outputs"])
	if err != nil {
		return nil, 0, 0, err
	}
	lengths, _, err := ort.GetTensorData[int64](outputs["encoded_lengths"])
	if err != nil || len(shape) != 3 || len(lengths) != 1 {
		return nil, 0, 0, fmt.Errorf("the encoder answered with shape %v: %v", shape, err)
	}
	dims, stride := int(shape[1]), int(shape[2])
	frames := min(int(lengths[0]), stride)
	// Copy frame by frame into rows of dims values, the layout the decoder takes one frame at a time.
	rows := make([]float32, frames*dims)
	for t := range frames {
		for d := range dims {
			rows[t*dims+d] = encoded[d*stride+t]
		}
	}
	return rows, dims, frames, nil
}

// decode runs the token-and-duration transducer greedily over the encoded frames.
func (m *Model) decode(ctx context.Context, encoded []float32, dims, frames int) ([]int, error) {
	stateSize := stateLayers * stateUnits
	state1, state2 := make([]float32, stateSize), make([]float32, stateSize)
	var tokens []int
	emitted := 0
	for t := 0; t < frames; {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		previous := m.blank
		if len(tokens) > 0 {
			previous = tokens[len(tokens)-1]
		}
		scores, next1, next2, err := m.step(ctx, encoded[t*dims:(t+1)*dims], previous, state1, state2)
		if err != nil {
			return nil, err
		}
		vocabularySize := len(m.vocabulary)
		token := argmax(scores[:vocabularySize])
		duration := argmax(scores[vocabularySize:])
		if token != m.blank {
			state1, state2 = next1, next2
			tokens = append(tokens, token)
			emitted++
		}
		switch {
		case duration > 0:
			t += duration
			emitted = 0
		case token == m.blank || emitted == maxTokensPerFrame:
			t++
			emitted = 0
		}
	}
	return tokens, nil
}

// step runs the prediction and joint networks on one encoder frame.
func (m *Model) step(ctx context.Context, frame []float32, previous int, state1, state2 []float32) ([]float32, []float32, []float32, error) {
	inputs := map[string]*ort.Value{}
	defer func() {
		for _, value := range inputs {
			value.Close()
		}
	}()
	add := func(name string, value *ort.Value, err error) error {
		if err != nil {
			return err
		}
		inputs[name] = value
		return nil
	}
	stateShape := []int64{stateLayers, 1, stateUnits}
	err := errors.Join(
		add(tensor(m.runtime, frame, []int64{1, int64(len(frame)), 1})("encoder_outputs")),
		add(tensor(m.runtime, []int32{int32(previous)}, []int64{1, 1})("targets")),
		add(tensor(m.runtime, []int32{1}, []int64{1})("target_length")),
		add(tensor(m.runtime, state1, stateShape)("input_states_1")),
		add(tensor(m.runtime, state2, stateShape)("input_states_2")),
	)
	if err != nil {
		return nil, nil, nil, err
	}
	outputs, err := m.decoder.Run(ctx, inputs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode the speech: %w", err)
	}
	for _, value := range outputs {
		defer value.Close()
	}
	scores, _, err1 := ort.GetTensorData[float32](outputs["outputs"])
	next1, _, err2 := ort.GetTensorData[float32](outputs["output_states_1"])
	next2, _, err3 := ort.GetTensorData[float32](outputs["output_states_2"])
	if err := errors.Join(err1, err2, err3); err != nil {
		return nil, nil, nil, err
	}
	return append([]float32(nil), scores...), append([]float32(nil), next1...), append([]float32(nil), next2...), nil
}

// tensor returns a function that names a new tensor, so step can build its inputs in one list.
func tensor[T ort.TensorData](rt *ort.Runtime, data []T, shape []int64) func(string) (string, *ort.Value, error) {
	return func(name string) (string, *ort.Value, error) {
		value, err := ort.NewTensorValue(rt, data, shape)
		return name, value, err
	}
}

func argmax(values []float32) int {
	best := 0
	for i, value := range values {
		if value > values[best] {
			best = i
		}
	}
	return best
}

// detokenize joins the word pieces. A piece that starts a word carries a space; a space before punctuation or at the
// very start is dropped.
func (m *Model) detokenize(tokens []int) string {
	var joined strings.Builder
	for _, token := range tokens {
		joined.WriteString(m.vocabulary[token])
	}
	runes := []rune(joined.String())
	var text strings.Builder
	for i, r := range runes {
		if unicode.IsSpace(r) {
			if text.Len() == 0 || i+1 >= len(runes) || !isWordRune(runes[i+1]) {
				continue
			}
		}
		text.WriteRune(r)
	}
	return text.String()
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// readVocabulary reads "piece id" lines into a table by id, with word starts turned into spaces.
func readVocabulary(path string) ([]string, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read the speech vocabulary: %w", err)
	}
	defer file.Close()
	byID := map[int]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		split := strings.LastIndexByte(line, ' ')
		if split < 0 {
			continue
		}
		id, err := strconv.Atoi(line[split+1:])
		if err != nil {
			return nil, 0, fmt.Errorf("the speech vocabulary has a bad line %q", line)
		}
		byID[id] = strings.ReplaceAll(line[:split], wordStartCharacter, " ")
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}
	vocabulary := make([]string, len(byID))
	blank := -1
	for id, piece := range byID {
		if id < 0 || id >= len(vocabulary) {
			return nil, 0, errors.New("the speech vocabulary skips ids")
		}
		vocabulary[id] = piece
		if piece == blankToken {
			blank = id
		}
	}
	if blank < 0 {
		return nil, 0, errors.New("the speech vocabulary has no blank token")
	}
	return vocabulary, blank, nil
}
