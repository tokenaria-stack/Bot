package server

import "testing"

func TestDebugFlags_DefaultOff(t *testing.T) {
	// Process may inherit env; force-clear for this assertion.
	prevT, prevP := DebugTipSSOT(), DebugProjCont()
	t.Cleanup(func() {
		SetDebugTipSSOT(prevT)
		SetDebugProjCont(prevP)
	})
	SetDebugTipSSOT(false)
	SetDebugProjCont(false)
	if DebugTipSSOT() || DebugProjCont() {
		t.Fatal("expected debug probes off after explicit disable")
	}
	SetDebugTipSSOT(true)
	SetDebugProjCont(true)
	if !DebugTipSSOT() || !DebugProjCont() {
		t.Fatal("expected debug probes on after enable")
	}
}
