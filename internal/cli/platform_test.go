package cli

import "testing"

func TestSupported(t *testing.T) {
	if err := Supported("windows"); err == nil {
		t.Error("windows must be unsupported")
	}
	for _, goos := range []string{"linux", "darwin"} {
		if err := Supported(goos); err != nil {
			t.Errorf("Supported(%q) = %v, want nil", goos, err)
		}
	}
}
