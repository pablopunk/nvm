package api

import "testing"

func TestParseVersionReadsMajorAndMinor(t *testing.T) {
	version, err := ParseVersion("1.2")
	if err != nil || version != (Version{Major: 1, Minor: 2}) {
		t.Fatalf("ParseVersion(1.2) = %v, %v", version, err)
	}
	for _, bad := range []string{"", "1", "a.b", "1.-1", "-1.0", "1.2.3x"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("ParseVersion(%q) accepted a bad version", bad)
		}
	}
}

func TestCheckCompatibilityAcceptsSameMajorAndOlderMinor(t *testing.T) {
	host := Version{Major: 1, Minor: 3}
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{name: "same version", target: "1.3"},
		{name: "older minor", target: "1.0"},
		{name: "newer minor is rejected", target: "1.4", wantErr: true},
		{name: "next major is rejected", target: "2.0", wantErr: true},
		{name: "previous major is rejected", target: "0.9", wantErr: true},
		{name: "malformed is rejected", target: "one", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckCompatibility(tt.target, host)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckCompatibility(%q) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
		})
	}
}

func TestCurrentVersionIsStableAndFormats(t *testing.T) {
	if Current.String() != "1.0" {
		t.Fatalf("Current = %s, want 1.0 for the first design", Current)
	}
}
