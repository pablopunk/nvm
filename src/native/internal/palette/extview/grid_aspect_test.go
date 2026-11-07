package extview

import "testing"

func TestGridAspectRatio(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  float32
	}{{"16/9", 16.0 / 9}, {"1/1", 1}, {"bad", 16.0 / 9}, {"1/0", 16.0 / 9}} {
		if got := gridAspectRatio(tc.input); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.input, got, tc.want)
		}
	}
}
