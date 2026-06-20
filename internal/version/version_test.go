package version

import "testing"

func TestPhase(t *testing.T) {
	if Phase != "0" {
		t.Fatalf("Phase = %q, want \"0\"", Phase)
	}
}
