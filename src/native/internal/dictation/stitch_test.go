package dictation

import "testing"

func TestStitchTakesTheSeamFromThePieceThatHeardWhatFollowed(t *testing.T) {
	cases := []struct {
		name, sofar, next string
		overlap           int
		want              string
	}{
		{
			name:    "a pause mid-sentence loses its period",
			sofar:   "And so, my fellow Americans, ask not.",
			next:    "fellow Americans, ask not what your country can do for you,",
			overlap: 4,
			want:    "And so, my fellow Americans, ask not what your country can do for you,",
		},
		{
			name:    "a real sentence end keeps its period",
			sofar:   "Ask what you can do for your country.",
			next:    "for your country. This is the micro machine man",
			overlap: 4,
			want:    "Ask what you can do for your country. This is the micro machine man",
		},
		{
			name:    "a word the overlap cut in half is dropped",
			sofar:   "We choose to go to the moon.",
			next:    "oose to go to the moon in this decade",
			overlap: 6,
			want:    "We choose to go to the moon in this decade",
		},
		{
			name:    "case and punctuation do not hide the seam",
			sofar:   "Send it to Maria tomorrow.",
			next:    "maria tomorrow, and copy Pablo",
			overlap: 2,
			want:    "Send it to Maria tomorrow, and copy Pablo",
		},
		{
			name:    "a sentence end the next piece heard keeps its case consistent",
			sofar:   "And so, my fellow Americans, ask not.",
			next:    "Americans. Ask not what your country",
			overlap: 4,
			want:    "And so, my fellow Americans. Ask not what your country",
		},
		{
			name:    "accented words merge whole",
			sofar:   "Vamos a la acción.",
			next:    "la acción y después comemos",
			overlap: 2,
			want:    "Vamos a la acción y después comemos",
		},
		{
			name:    "without a seam the overlap's likely words are dropped",
			sofar:   "one two three four",
			next:    "xx yy then five six",
			overlap: 2,
			want:    "one two three four then five six",
		},
		{
			name:    "an empty text so far takes the piece as it is",
			sofar:   "",
			next:    "hello there",
			overlap: 3,
			want:    "hello there",
		},
		{
			name:    "a short common word alone is not a seam",
			sofar:   "put it in the",
			next:    "the box on the left",
			overlap: 0,
			want:    "put it in the the box on the left",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stitch(c.sofar, c.next, c.overlap); got != c.want {
				t.Fatalf("stitch = %q\n                want %q", got, c.want)
			}
		})
	}
}

func TestLivePiecesRepeatTheEndOfThePieceBefore(t *testing.T) {
	h := newHarness(t)
	h.transcriber.bySegment = map[int]string{0: "first sentence.", 1: "first sentence, second sentence."}
	h.mic.Queue(voicedWithPause(6, 2)...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	text, err := h.service.Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if text != "first sentence, second sentence." {
		t.Fatalf("text = %q; want the seam taken from the second piece", text)
	}
	files := h.transcriber.files()
	if len(files) != 2 || len(files[1]) <= len(files[0])/3 {
		t.Fatalf("got %d files; want the second to carry the overlap", len(files))
	}
}
