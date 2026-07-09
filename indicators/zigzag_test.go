package indicators

import "testing"

func TestZigZagSaveRestoreOpenBar(t *testing.T) {
	zz := NewZigZag(DefaultATRPeriod)
	zz.SetSensitivity(0.5)

	for i := 0; i < 30; i++ {
		p := 100.0 + float64(i)*0.1
		zz.UpdateCandle(p+0.2, p-0.2, p, 50)
		zz.SaveState()
	}

	beforeNode := zz.lastNode
	beforeDir := zz.direction

	// Simulate open-bar structural mutation.
	zz.lastNode = ZigZagNode{Price: 999, IsHigh: true, Confirmed: true}
	zz.direction = ZigZagUp
	if wf, ok := zz.fractals.(*WilliamsFractals); ok {
		wf.idx = (wf.idx + 1) % 5
	}

	zz.RestoreState()
	if zz.lastNode != beforeNode {
		t.Fatalf("zigzag node not restored: got %+v want %+v", zz.lastNode, beforeNode)
	}
	if zz.direction != beforeDir {
		t.Fatalf("zigzag direction not restored: got %v want %v", zz.direction, beforeDir)
	}
}
