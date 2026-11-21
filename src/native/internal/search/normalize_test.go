package search

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Loom", "loom"},
		{"  Google \t Chrome\n", "google chrome"},
		{"Café Olé", "cafe ole"},
		{"MAÑANA", "manana"},
		{"東京タワー", "東京タワー"},
		{"", ""},
		{" \t ", ""},
	}
	for _, tt := range tests {
		if got := string(normalize(nil, tt.in)); got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
