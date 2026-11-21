package app

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestAvailableOSRelease(t *testing.T) {
	tests := []struct {
		name   string
		source *fake.OSRelease
		want   string
	}{
		{name: "available", source: &fake.OSRelease{Release: "24.6.0"}, want: "24.6.0"},
		{name: "error", source: &fake.OSRelease{Release: "24.6.0", Err: errors.New("unavailable")}},
		{name: "empty", source: &fake.OSRelease{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := availableOSRelease(test.source); got != test.want {
				t.Fatalf("availableOSRelease() = %q, want %q", got, test.want)
			}
		})
	}
	if got := availableOSRelease(nil); got != "" {
		t.Fatalf("availableOSRelease(nil) = %q, want empty", got)
	}
}
