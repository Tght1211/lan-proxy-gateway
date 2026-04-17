package cmd

import "testing"

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		{name: "newer tag", latest: "v2.2.0", current: "v2.1.1", want: true},
		{name: "equal tag", latest: "v2.2.0", current: "v2.2.0", want: false},
		{name: "older tag", latest: "v2.1.0", current: "v2.2.0", want: false},
		{name: "missing prefix", latest: "2.2.0", current: "v2.1.1", want: true},
		{name: "invalid version", latest: "latest", current: "v2.1.1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNewerVersion(tt.latest, tt.current); got != tt.want {
				t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}
