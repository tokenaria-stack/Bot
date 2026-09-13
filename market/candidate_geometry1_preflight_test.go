package market

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
)

func TestCandidateGeometry1_ResearchArchive(t *testing.T) {
	if os.Getenv("CANDIDATE_GEOMETRY_1") != "1" {
		t.Skip("set CANDIDATE_GEOMETRY_1=1 to run the archive geometry study")
	}

	spec2, err := ResearchFeatureSpec2()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	tapeName, err := ResearchFeatureTape2FileName(spec2)
	if err != nil {
		t.Fatal(err)
	}
	tapePath := filepath.Join(root, "research", "tapes2", tapeName)
	th, trows, _, err := forecast.ReadTape2(tapePath)
	if err != nil {
		t.Fatalf("ReadTape2: %v (need FEATURE_TAPE2_PREFLIGHT tape)", err)
	}

	primary := loadResearchClosedBarsTF(t, "15m")
	h1 := loadResearchClosedBarsTF(t, "1h")
	finer := loadResearchClosedBarsTF(t, "1m")
	pCanon, err := klinesToCanonical(primary)
	if err != nil {
		t.Fatal(err)
	}
	hCanon, err := klinesToCanonical(h1)
	if err != nil {
		t.Fatal(err)
	}
	fCanon, err := klinesToCanonical(finer)
	if err != nil {
		t.Fatal(err)
	}
	fm := th.Primary
	fm.Timeframe = "1m"

	in := forecast.CandidateGeometryInput{
		PrimaryTF: "15m", FinerTF: "1m", HTF1hTF: "1h",
		FinerMarket: fm,
		Primary:     pCanon,
		HTF1h:       hCanon,
		Finer:       fCanon,
		Tape:        trows,
	}
	a, err := forecast.RunCandidateGeometry1(in)
	if err != nil {
		t.Fatalf("run A: %v", err)
	}
	b, err := forecast.RunCandidateGeometry1(in)
	if err != nil {
		t.Fatalf("run B: %v", err)
	}
	sa, sb := forecast.FormatCandidateGeometry1(a), forecast.FormatCandidateGeometry1(b)
	if sa != sb {
		t.Fatal("determinism: two runs produced different reports")
	}
	if len(a.Cells) != 32 {
		t.Fatalf("cells=%d", len(a.Cells))
	}
	fmt.Fprint(os.Stderr, sa)
	fmt.Fprintf(os.Stderr, "\nCANDIDATE-GEOMETRY-1 GREEN cells=32 pops=%d NO TARGET SELECTED\n", len(a.Populations))
}
