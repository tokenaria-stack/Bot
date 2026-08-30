package indicators

import "testing"

func TestHighestBarsAgo_XbarsWindow(t *testing.T) {
	t.Parallel()
	n := 60
	rsx := make([]float64, n)
	for i := 0; i < n; i++ {
		rsx[i] = 40
	}
	rsx[5] = 90
	far := highestBarsAgo(rsx, 50, 90)
	near := highestBarsAgo(rsx, 50, 8)
	if far == near {
		t.Fatalf("xbars must change highestbars: 90=%d 8=%d", far, near)
	}
	if far != 45 {
		t.Fatalf("lookback 90 peak at bar 5: ago=%d", far)
	}
}
