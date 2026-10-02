package version

import (
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		ok       bool
		want     string
	}{
		{name: "injected wins", injected: "v0.2.1", info: nil, ok: false, want: "0.2.1"},
		{name: "buildinfo used when not injected", injected: "", info: &debug.BuildInfo{Main: debug.Module{Version: "v0.3.0"}}, ok: true, want: "0.3.0"},
		{name: "devel falls back to dev", injected: "", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ok: true, want: "dev"},
		{name: "empty buildinfo falls back to dev", injected: "", info: &debug.BuildInfo{Main: debug.Module{Version: ""}}, ok: true, want: "dev"},
		{name: "no buildinfo falls back to dev", injected: "", info: nil, ok: false, want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolve(tt.injected, tt.info, tt.ok); got != tt.want {
				t.Errorf("resolve(%q, %v, %v) = %q, want %q", tt.injected, tt.info, tt.ok, got, tt.want)
			}
		})
	}
}
