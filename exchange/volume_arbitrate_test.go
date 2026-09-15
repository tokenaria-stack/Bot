package exchange

import "testing"

func TestSumChildBase_FifteenMinutes(t *testing.T) {
	t.Parallel()
	parent := int64(1_700_000_000_000)
	var kids []VolumeAuthority
	var want float64
	for i := 0; i < 15; i++ {
		v := float64(i + 1)
		want += v
		kids = append(kids, VolumeAuthority{
			OpenTime: parent + int64(i)*60_000,
			Open:     float64(i), High: float64(i) + 1, Low: float64(i) - 1, Close: float64(i) + 0.5,
			Base: v, Quote: v * 10, Trades: int64(i + 2),
		})
	}
	// outsider
	kids = append(kids, VolumeAuthority{OpenTime: parent + parent15mMs, Base: 999})
	n, vol, q, tr, o, h, l, c := SumChildBase(parent, parent15mMs, kids)
	if n != 15 || vol != want || q != want*10 || tr != 2+3+4+5+6+7+8+9+10+11+12+13+14+15+16 {
		t.Fatalf("n=%d vol=%v q=%v tr=%d want vol=%v", n, vol, q, tr, want)
	}
	if o != 0 || h != 15 || l != -1 || c != 14.5 {
		t.Fatalf("ohlc %v %v %v %v", o, h, l, c)
	}
}

func TestClassify15mChildSupport(t *testing.T) {
	t.Parallel()
	if Classify15mChildSupport(100, 90, 15, 15, 100, 90) != SupportBothInternal {
		t.Fatal("both")
	}
	if Classify15mChildSupport(100, 90, 15, 15, 100, 80) != SupportRESTOnly {
		t.Fatal("rest")
	}
	if Classify15mChildSupport(100, 90, 15, 15, 80, 90) != SupportVisionOnly {
		t.Fatal("vision")
	}
	if Classify15mChildSupport(100, 90, 15, 15, 80, 70) != SupportNeither {
		t.Fatal("neither")
	}
	if Classify15mChildSupport(100, 90, 14, 15, 100, 90) != SupportVisionOnly {
		t.Fatal("vis complete rest incomplete → vision only")
	}
	if Classify15mChildSupport(100, 90, 14, 14, 90, 80) != SupportIncomplete {
		t.Fatal("incomplete")
	}
}
