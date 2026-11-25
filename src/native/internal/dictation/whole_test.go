package dictation

import (
	"context"
	"testing"
)

// scriptedReader answers the whole reading and the end check with scripted texts, by the length it is asked to read.
type scriptedReader struct {
	whole, tail string
	full        int
}

func (r scriptedReader) TranscribePCM(_ context.Context, pcm []int16) (string, error) {
	if len(pcm) == r.full {
		return r.whole, nil
	}
	return r.tail, nil
}

func TestTheEndCheckAddsALastSentenceTheWholeReadingLeftOut(t *testing.T) {
	pcm := make([]int16, 40*SampleRate)
	cases := []struct{ name, whole, tail, want string }{
		{
			name:  "a dropped last sentence is added",
			whole: "One two three four five six seven eight nine ten. Something is definitely wrong.",
			tail:  "nine ten. Something is definitely wrong. End of communication.",
			want:  "One two three four five six seven eight nine ten. Something is definitely wrong. End of communication.",
		},
		{
			name:  "a complete reading is left as it is",
			whole: "One two three four five six seven eight nine ten. End of communication.",
			tail:  "nine ten. End of communication.",
			want:  "One two three four five six seven eight nine ten. End of communication.",
		},
		{
			name:  "an end check that matches nothing adds nothing",
			whole: "One two three four five six seven eight nine ten.",
			tail:  "zz yy",
			want:  "One two three four five six seven eight nine ten.",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := readWhole(context.Background(), scriptedReader{whole: c.whole, tail: c.tail, full: len(pcm)}, RetryPolicy{Sleep: noWait}, pcm, nil)
			if err != nil || got != c.want {
				t.Fatalf("readWhole = %q, %v\n                 want %q", got, err, c.want)
			}
		})
	}
}

func TestAShortDictationIsNotReadTwice(t *testing.T) {
	calls := 0
	reader := countingReader{calls: &calls}
	if _, err := readWhole(context.Background(), reader, RetryPolicy{Sleep: noWait}, make([]int16, 10*SampleRate), nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("read %d times; want one reading for a dictation shorter than the end check", calls)
	}
}

type countingReader struct{ calls *int }

func (r countingReader) TranscribePCM(context.Context, []int16) (string, error) {
	*r.calls++
	return "hello", nil
}

func TestTheEndCheckStartsInAPause(t *testing.T) {
	pcm := make([]int16, 30*SampleRate)
	for i := range pcm {
		pcm[i] = 9000
	}
	quiet := 12 * SampleRate
	for i := quiet; i < quiet+SampleRate/2; i++ {
		pcm[i] = 0
	}
	if start := quietestStart(pcm, 10*SampleRate, 15*SampleRate); start < quiet || start >= quiet+SampleRate/2 {
		t.Fatalf("end check starts at %.2f s; want inside the pause at 12 s", float64(start)/SampleRate)
	}
}
