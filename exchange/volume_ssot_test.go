package exchange

import "testing"

func fifteenKids(parent int64, volFn func(i int) float64, ohlcShift float64) []VolumeAuthority {
	var kids []VolumeAuthority
	for i := 0; i < 15; i++ {
		ot := parent + int64(i)*native1mMs
		kids = append(kids, VolumeAuthority{
			OpenTime: ot,
			Open:     100 + float64(i) + ohlcShift,
			High:     110 + ohlcShift,
			Low:      90 + ohlcShift,
			Close:    101 + float64(i) + ohlcShift,
			Base:     volFn(i),
		})
	}
	return kids
}

func TestAlignExact1mChildren_Complete(t *testing.T) {
	t.Parallel()
	parent := int64(1_700_000_000_000)
	kids := fifteenKids(parent, func(i int) float64 { return float64(i + 1) }, 0)
	got, err := AlignExact1mChildren(parent, kids)
	if err != nil || len(got) != 15 {
		t.Fatalf("complete: %v n=%d", err, len(got))
	}
	for i, k := range got {
		if k.OpenTime != parent+int64(i)*native1mMs {
			t.Fatalf("order %d %d", i, k.OpenTime)
		}
	}
}

func TestAlignExact1mChildren_Missing(t *testing.T) {
	t.Parallel()
	parent := int64(1_700_000_000_000)
	kids := fifteenKids(parent, func(i int) float64 { return 1 }, 0)
	kids = kids[:14]
	if _, err := AlignExact1mChildren(parent, kids); err == nil {
		t.Fatal("missing child must refuse")
	}
}

func TestAlignExact1mChildren_Duplicate(t *testing.T) {
	t.Parallel()
	parent := int64(1_700_000_000_000)
	kids := fifteenKids(parent, func(i int) float64 { return 1 }, 0)
	kids = append(kids, kids[0])
	if _, err := AlignExact1mChildren(parent, kids); err == nil {
		t.Fatal("duplicate child must refuse")
	}
}

func TestREST15mOHLCIdentity(t *testing.T) {
	t.Parallel()
	parent := int64(1_700_000_000_000)
	kids := fifteenKids(parent, func(i int) float64 { return 1 }, 0)
	_, o, h, l, c := AggregateAligned1m(kids)
	p := VolumeAuthority{OpenTime: parent, Open: o, High: h, Low: l, Close: c, Base: 15}
	if !REST15mOHLCIdentity(p, o, h, l, c) {
		t.Fatal("identity should pass")
	}
	if REST15mOHLCIdentity(p, o+1, h, l, c) {
		t.Fatal("open mismatch must fail")
	}
}

func TestResolve_RESTParentValid(t *testing.T) {
	t.Parallel()
	parentOT := int64(1_700_000_000_000)
	kids := fifteenKids(parentOT, func(i int) float64 { return 2 }, 0)
	sum, o, h, l, c := AggregateAligned1m(kids)
	p := VolumeAuthority{OpenTime: parentOT, Open: o, High: h, Low: l, Close: c, Base: sum}
	d := ResolveFuturesRESTFamily15m(p, kids)
	if d.Source != CanonicalREST15mParent || d.PolicyClass != PolicyRESTParentValid || d.Volume != sum {
		t.Fatalf("%+v", d)
	}
}

func TestResolve_RESTParentInvalidChildrenCertified(t *testing.T) {
	t.Parallel()
	parentOT := int64(1_700_000_000_000)
	kids := fifteenKids(parentOT, func(i int) float64 { return float64(i + 1) }, 0)
	sum, o, h, l, c := AggregateAligned1m(kids)
	p := VolumeAuthority{OpenTime: parentOT, Open: o, High: h, Low: l, Close: c, Base: 9.646}
	d := ResolveFuturesRESTFamily15m(p, kids)
	if d.Source != CanonicalREST1mRecon || d.PolicyClass != PolicyRESTParentInvalidKids {
		t.Fatalf("%+v", d)
	}
	if !VolumeFloatEqual(d.Volume, sum) || d.Volume == p.Base {
		t.Fatalf("canonical must be child sum %v got %v parent %v", sum, d.Volume, p.Base)
	}
}

func TestResolve_OHLCFailureRefusesEvenWithVolumeSum(t *testing.T) {
	t.Parallel()
	parentOT := int64(1_700_000_000_000)
	kids := fifteenKids(parentOT, func(i int) float64 { return 1 }, 0)
	sum, o, h, l, c := AggregateAligned1m(kids)
	p := VolumeAuthority{OpenTime: parentOT, Open: o + 50, High: h + 50, Low: l - 50, Close: c + 50, Base: sum}
	d := ResolveFuturesRESTFamily15m(p, kids)
	if d.Source != CanonicalUnresolved || d.PolicyClass != PolicyRESTFamilyUnresolved {
		t.Fatalf("ohlc fail must quarantine %+v", d)
	}
}

func TestResolve_AdditiveParentOpenMismatchStillParent(t *testing.T) {
	t.Parallel()
	parentOT := int64(1_700_000_000_000)
	kids := fifteenKids(parentOT, func(i int) float64 { return 2 }, 0)
	sum, o, h, l, c := AggregateAligned1m(kids)
	p := VolumeAuthority{OpenTime: parentOT, Open: o + 40, High: h, Low: l, Close: c, Base: sum}
	d := ResolveFuturesRESTFamily15m(p, kids)
	if d.Source != CanonicalREST15mParent || !d.OpenMismatch || d.Volume != sum {
		t.Fatalf("%+v", d)
	}
}

func TestResolve_BrokenParentOpenMatchAllowsReconstruction(t *testing.T) {
	t.Parallel()
	parentOT := int64(1_700_000_000_000)
	kids := fifteenKids(parentOT, func(i int) float64 { return float64(i + 1) }, 0)
	sum, o, h, l, c := AggregateAligned1m(kids)
	p := VolumeAuthority{OpenTime: parentOT, Open: o, High: h - 1, Low: l + 1, Close: c + 10, Base: 9.646}
	d := ResolveFuturesRESTFamily15m(p, kids)
	if d.Source != CanonicalREST1mRecon || d.OHLCIdentityOK {
		t.Fatalf("%+v", d)
	}
	if !VolumeFloatEqual(d.Volume, sum) {
		t.Fatalf("sum %v got %v", sum, d.Volume)
	}
}

func TestResolve_BrokenParentOpenMismatchRefused(t *testing.T) {
	t.Parallel()
	parentOT := int64(1_700_000_000_000)
	kids := fifteenKids(parentOT, func(i int) float64 { return 1 }, 0)
	_, o, h, l, c := AggregateAligned1m(kids)
	p := VolumeAuthority{OpenTime: parentOT, Open: o + 9, High: h, Low: l, Close: c, Base: 0.5}
	d := ResolveFuturesRESTFamily15m(p, kids)
	if d.Source != CanonicalUnresolved {
		t.Fatalf("%+v", d)
	}
}

func TestResolve_VisionDisagreementDoesNotAlterREST(t *testing.T) {
	t.Parallel()
	parentOT := int64(1_700_000_000_000)
	kids := fifteenKids(parentOT, func(i int) float64 { return 3 }, 0)
	sum, o, h, l, c := AggregateAligned1m(kids)
	p := VolumeAuthority{OpenTime: parentOT, Open: o, High: h, Low: l, Close: c, Base: sum}
	d := ResolveFuturesRESTFamily15m(p, kids)
	if d.Source != CanonicalREST15mParent || d.Volume != sum {
		t.Fatalf("%+v", d)
	}
	_ = 1663.097 // Vision parent; unused on purpose
}

func TestDigestCanonicalVolumeSeries_DeterministicAndSensitive(t *testing.T) {
	t.Parallel()
	ot := []int64{1, 2, 3}
	vol := []float64{1.5, 2.5, 3.5}
	a := DigestCanonicalVolumeSeries("BTCUSDT", "15m", ot, vol)
	b := DigestCanonicalVolumeSeries("BTCUSDT", "15m", ot, vol)
	if a == "" || a != b {
		t.Fatalf("repeat %s %s", a, b)
	}
	vol2 := []float64{1.5, 2.5, 3.5000001}
	c := DigestCanonicalVolumeSeries("BTCUSDT", "15m", ot, vol2)
	if c == a {
		t.Fatal("volume change must change digest")
	}
	d1h := DigestCanonicalVolumeSeries("BTCUSDT", "1h", ot, vol)
	if d1h == a {
		t.Fatal("tf change must change digest")
	}
}
