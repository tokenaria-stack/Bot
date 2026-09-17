package market

import (
	"testing"
)

func TestDAGRunner_NoLegacyScoreChain(t *testing.T) {
	t.Parallel()
	r := newDAGRunner(64, NormalizeRSXSettings(RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"}))
	if r.NodeByName("divergence") != nil {
		t.Fatal("DivergenceNode must not be registered")
	}
	if r.NodeByName("micro_pattern") != nil || r.NodeByName("score") != nil {
		t.Fatal("MicroPatternNode / ScoreNode must not be registered")
	}
	if r.NodeByName("rsx") == nil || r.NodeByName("wozduh") == nil || r.NodeByName("zigzag") == nil {
		t.Fatal("rsx/wozduh/zigzag must remain")
	}
}
