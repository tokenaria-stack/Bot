package nodes

import "testing"

func TestDetectVolCrossCode(t *testing.T) {
	t.Parallel()
	if got := detectVolCrossCode(40, 50, 55, 48, true); got != wozduhVolCrossLime {
		t.Fatalf("bull cross = %v, want lime %v", got, wozduhVolCrossLime)
	}
	if got := detectVolCrossCode(55, 48, 40, 50, true); got != wozduhVolCrossRed {
		t.Fatalf("bear cross = %v, want red %v", got, wozduhVolCrossRed)
	}
	if got := detectVolCrossCode(40, 50, 45, 48, false); got != wozduhVolCrossNone {
		t.Fatalf("not ready = %v, want none", got)
	}
}
