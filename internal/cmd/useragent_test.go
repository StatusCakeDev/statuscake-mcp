package cmd

import (
	"fmt"
	"runtime"
	"testing"
)

func TestUserAgent(t *testing.T) {
	t.Parallel()

	platform := fmt.Sprintf("(%s; %s; %s)", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	tests := map[string]struct {
		version string
		commit  string
		want    string
	}{
		"release": {
			version: "1.2.3",
			want:    "statuscake-mcp/1.2.3 " + platform,
		},
		"release with commit": {
			version: "1.2.3",
			commit:  "abc1234",
			want:    "statuscake-mcp/1.2.3 (abc1234) " + platform,
		},
		"unset version": {
			want: "statuscake-mcp/dev " + platform,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := userAgent(tt.version, tt.commit); got != tt.want {
				t.Errorf("userAgent(%q, %q) = %q, want %q", tt.version, tt.commit, got, tt.want)
			}
		})
	}
}
