package brain3

import (
	"math"
	"testing"
)

func TestSignedRankBiserial_Known(t *testing.T) {
	t.Parallel()
	sep := signedRankBiserial([]float64{3, 4, 5}, []float64{0, 1, 2})
	if math.Abs(sep-1) > 1e-12 {
		t.Fatal(sep)
	}
	neg := signedRankBiserial([]float64{0, 1}, []float64{8, 9})
	if math.Abs(neg+1) > 1e-12 {
		t.Fatal(neg)
	}
	z := signedRankBiserial([]float64{1, 1}, []float64{1, 1})
	if z != 0 {
		t.Fatal(z)
	}
}

func TestToBinaryY(t *testing.T) {
	t.Parallel()
	if y, ok := toBinaryY(0); !ok || y != 1 {
		t.Fatal("TP")
	}
	if y, ok := toBinaryY(1); !ok || y != 0 {
		t.Fatal("STOP")
	}
	if _, ok := toBinaryY(2); ok {
		t.Fatal("TIMEOUT")
	}
}

func TestBinarySpec_DigestStable(t *testing.T) {
	t.Parallel()
	ds := Dataset{ContentHex: "a", FeatureNames: []string{"x"}, Width: 1}
	val := Validation{PlanHex: "p", SplitHex: "s"}
	s := BinaryProbeSpec1()
	h1, err := s.DigestHex(ds, val)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := s.DigestHex(ds, val)
	if err != nil || h1 != h2 {
		t.Fatal(h1, h2, err)
	}
}

func TestFeatureFamily(t *testing.T) {
	t.Parallel()
	if featureFamily("rsx_delta_1") != "rsx_core" || featureFamily("tv_bear_present") != "tv" {
		t.Fatal("family")
	}
	if featureFamily("setup_risk_atr") != "setup" || featureFamily("htf_1h_rsx_value") != "htf_1h" {
		t.Fatal("setup/htf")
	}
}
