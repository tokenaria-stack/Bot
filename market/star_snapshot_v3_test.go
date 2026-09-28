package market

import (
	"encoding/json"
	"reflect"
	"testing"

	"trading_bot/data"
)

func TestDailyCausalBoundaries(t *testing.T) {
	origin := starFixtureOrigin
	day := int64(24 * 60 * 60 * 1000)
	opens := []int64{origin, origin + day, origin + 2*day}
	s := blankStarSeries(len(opens))
	for i, open := range opens {
		closeAt, err := data.BarCloseTimeMs(open, "1d")
		if err != nil {
			t.Fatal(err)
		}
		s.open[i] = open
		s.close[i] = closeAt
		s.vwema[i] = 0
		s.mid[i] = 50
		s.up[i] = 51
		s.dn[i] = 49
		s.rsx[i] = 0
		s.sig[i] = 1
	}

	before, err := readCausalTF(s, s.close[0]-1)
	if err != nil {
		t.Fatal(err)
	}
	if before.Present || before.Vwema != 0 || before.ValuesOK {
		t.Fatalf("bar still open: %+v", before)
	}

	exact, err := readCausalTF(s, s.close[0])
	if err != nil {
		t.Fatal(err)
	}
	if !exact.Present || exact.OpenTime != opens[0] || exact.CloseTime != s.close[0] {
		t.Fatalf("exact close: %+v", exact)
	}
	if !exact.ValuesOK || exact.Vwema != 0 {
		t.Fatalf("zero reading stored as missing: %+v", exact)
	}
	rsx := projectStarRSX(s, 0)
	if !rsx.ValueOK || rsx.Value != 0 {
		t.Fatalf("zero rsx stored as missing: %+v", rsx)
	}

	after, err := readCausalTF(s, s.close[0]+1)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Present || after.OpenTime != opens[0] {
		t.Fatalf("after close: %+v", after)
	}

	stillFirst, err := readCausalTF(s, s.close[1]-1)
	if err != nil {
		t.Fatal(err)
	}
	if !stillFirst.Present || stillFirst.OpenTime != opens[0] {
		t.Fatalf("next bar still open: %+v", stillFirst)
	}

	next, err := readCausalTF(s, s.close[1])
	if err != nil {
		t.Fatal(err)
	}
	if !next.Present || next.OpenTime != opens[1] || next.CloseTime != s.close[1] {
		t.Fatalf("next close: %+v", next)
	}

	// The last 15m bar of the day opens before the daily close and closes on it.
	// Visibility follows that close. The open would hide this daily bar.
	starOpen := s.close[0] - 15*60*1000 + 1
	if latestCloseIndex(s.close, starOpen) != -1 {
		t.Fatal("15m open sees the daily bar that closes with that bar")
	}
	if latestCloseIndex(s.close, s.close[0]) != 0 {
		t.Fatal("15m close misses the daily bar that closes with that bar")
	}
}

func TestExtractStarSnapshotsV3(t *testing.T) {
	k15 := starFixtureKlines(16*160, "15m")
	k1h := starFixtureKlines(4*160, "1h")
	k4h := starFixtureKlines(160, "4h")
	k1d := starFixtureKlines(27, "1d")
	rsx := defaultRSXSettings()

	base, err := ExtractStarSnapshots(k15, k1h, k4h, rsx)
	if err != nil {
		t.Fatal(err)
	}
	before := testDAGRunnerBorn.Load()
	rows, err := ExtractStarSnapshotsV3(k15, k1h, k4h, k1d, rsx)
	if err != nil {
		t.Fatal(err)
	}
	if born := testDAGRunnerBorn.Load() - before; born != 4 {
		t.Fatalf("dag runners %d", born)
	}
	if len(rows) != len(base) || len(rows) < 2 {
		t.Fatalf("stars %d base %d", len(rows), len(base))
	}
	again, err := ExtractStarSnapshotsV3(k15, k1h, k4h, k1d, rsx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rows, again) {
		t.Fatal("daily extracts differ")
	}

	daily := loadBusSeries(t, k1d, "1d", rsx)
	var absent, exact, after int
	for i, row := range rows {
		if !reflect.DeepEqual(row.StarSnapshot, base[i]) {
			t.Fatalf("schema-2 fields moved at %d", row.AnchorAt)
		}
		if row.D1.Present && row.D1.CloseTime > row.M15.CloseTime {
			t.Fatalf("daily close %d after %d", row.D1.CloseTime, row.M15.CloseTime)
		}
		if !row.D1.Present && row.D1RSX != (StarRSX{}) {
			t.Fatalf("daily rsx without a bar at %d", row.AnchorAt)
		}
		assertHTFBus(t, "1d", row.M15.CloseTime, row.D1, row.D1RSX, daily)
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if jsonContains(raw, "D1TV") {
			t.Fatal("daily tv field")
		}
		switch {
		case !row.D1.Present:
			absent++
		case row.D1.CloseTime == row.M15.CloseTime:
			exact++
		default:
			after++
		}
	}
	if absent == 0 || after == 0 {
		t.Fatalf("boundaries absent=%d exact=%d after=%d", absent, exact, after)
	}

	far := int64(100000) * 24 * 60 * 60 * 1000
	shifted, err := ExtractStarSnapshotsV3(k15, k1h, k4h, shiftStarKlines(k1d, far), rsx)
	if err != nil {
		t.Fatal(err)
	}
	if len(shifted) != len(rows) {
		t.Fatalf("daily changed star count %d -> %d", len(rows), len(shifted))
	}
	for i, row := range shifted {
		if row.AnchorAt != rows[i].AnchorAt || row.Side != rows[i].Side || row.ConfirmedAt != rows[i].ConfirmedAt {
			t.Fatalf("shifted star %d", i)
		}
		if !reflect.DeepEqual(row.StarSnapshot, rows[i].StarSnapshot) {
			t.Fatalf("shifted schema-2 fields %d", i)
		}
		if row.D1.Present || row.D1RSX != (StarRSX{}) {
			t.Fatalf("future daily selected %+v", row.D1)
		}
	}
}

func jsonContains(raw []byte, key string) bool {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return true
	}
	_, ok := doc[key]
	return ok
}
