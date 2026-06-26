package strategy

import "testing"

func TestBaselineABMatrix_DisablesHTF(t *testing.T) {
	t.Parallel()

	base := ScoringMatrix{
		UseHTFOscillators: true,
		UseWozduhCross:    true,
	}
	got := BaselineABMatrix(base)
	if got.UseHTFOscillators {
		t.Fatal("baseline must disable HTF oscillators")
	}
	if !got.UseWozduhCross {
		t.Fatal("baseline should keep LTF toggles from base")
	}
}

func TestHTFRegimeABMatrix(t *testing.T) {
	t.Parallel()

	base := DefaultScoringMatrix()
	base.UseWozduhCross = true
	base.UseWozduhSpike = true
	base.UseRedCross = true

	got := HTFRegimeABMatrix(base)
	if !got.UseHTFOscillators {
		t.Fatal("HTF regime must enable HTF oscillators")
	}
	if got.UseWozduhCross || got.UseWozduhSpike || got.UseRedCross {
		t.Fatalf("noisy LTF should be off: %+v", got)
	}
}

func TestDefaultABRunSpecs(t *testing.T) {
	t.Parallel()

	specs := DefaultABRunSpecs(ScoringMatrix{UseRSX: true})
	if len(specs) != 2 {
		t.Fatalf("len = %d, want 2", len(specs))
	}
	if specs[1].Navigators["price"].Periods == nil {
		t.Fatal("HTF spec should enable navigator periods")
	}
}
