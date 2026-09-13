package forecast

import "testing"

func TestLastCausalK2_ProminenceIgnoresBarsAfterEntry(t *testing.T) {
	t.Parallel()
	const n = 40
	bars := make([]CanonicalClosedBar, n)
	for i := 0; i < n; i++ {
		bars[i] = CanonicalClosedBar{
			OpenTime: int64(i+1) * 900000, Open: 10, High: 11, Low: 9.5, Close: 10, Volume: 1,
		}
	}
	// Unique k=2 low at 20; confirm 22. Entry at 30.
	bars[20].Low = 5
	bars[20].High = 8
	for i := 23; i <= 30; i++ {
		bars[i].High = 12
	}
	bars[31].High = 80 // future — must not enter prominence
	entry := bars[30].OpenTime
	got, _, err := LastCausalK2WithProminence(bars, entry, true, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[0].ID != "S0" || got[0].AnchorAt != bars[20].OpenTime {
		t.Fatalf("%+v", got)
	}
	if got[0].FavorExtreme > 12.01 {
		t.Fatalf("used future high: %+v", got[0])
	}
}
