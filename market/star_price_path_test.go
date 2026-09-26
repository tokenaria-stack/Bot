package market

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"time"

	"trading_bot/core/nodes"
	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/indicators"
)

func TestPricePathExcludesStarAndUsesSuccessor(t *testing.T) {
	k15 := pricePathBars(12, 100)
	star := pricePathStar(t, k15, 4, nodes.WozduhXoverSideUp)
	got, err := BuildPricePath(star, k15, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.Complete != PathFilled || len(got.Bars) != 3 {
		t.Fatalf("complete %s bars %d", got.Complete, len(got.Bars))
	}
	for _, b := range got.Bars {
		if b.OpenTime == star.AnchorAt {
			t.Fatal("star bar entered the path")
		}
	}
	wantOpen, err := data.NextBarOpen(star.AnchorAt, "15m")
	if err != nil {
		t.Fatal(err)
	}
	if got.Bars[0].OpenTime != wantOpen || got.Bars[0].Open != k15[5].Open {
		t.Fatalf("first bar %+v", got.Bars[0])
	}
	if got.Signal != k15[4].Close || !got.FillOK || got.Fill != k15[5].Open {
		t.Fatalf("entries signal %v fill %v ok %v", got.Signal, got.Fill, got.FillOK)
	}
	if !got.GapOK || got.Gap != got.Fill-got.Signal {
		t.Fatalf("up gap %v", got.Gap)
	}
}

func TestPricePathDownGapAndNoFallback(t *testing.T) {
	k15 := pricePathBars(6, 50)
	star := pricePathStar(t, k15, 5, nodes.WozduhXoverSideDown)
	got, err := BuildPricePath(star, k15, 4)
	if err != nil {
		t.Fatal(err)
	}
	if got.Complete != PathTruncated || got.FillOK || got.GapOK || got.Fill != 0 || len(got.Bars) != 0 {
		t.Fatalf("truncated tail %+v", got)
	}
	if got.Signal != k15[5].Close {
		t.Fatalf("signal %v", got.Signal)
	}

	mid := pricePathStar(t, k15, 2, nodes.WozduhXoverSideDown)
	full, err := BuildPricePath(mid, k15, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !full.GapOK || full.Gap != full.Signal-full.Fill {
		t.Fatalf("down gap %v signal %v fill %v", full.Gap, full.Signal, full.Fill)
	}
}

func TestPricePathPrimaryGap(t *testing.T) {
	k15 := pricePathBars(8, 80)
	k15 = append(k15[:5], k15[6:]...)
	star := pricePathStar(t, k15, 4, nodes.WozduhXoverSideUp)
	got, err := BuildPricePath(star, k15, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.Complete != PathPrimaryGap || got.FillOK || len(got.Bars) != 0 {
		t.Fatalf("gap %+v", got)
	}

	early := pricePathStar(t, k15, 2, nodes.WozduhXoverSideUp)
	kept, err := BuildPricePath(early, k15, 4)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Complete != PathPrimaryGap || !kept.FillOK || len(kept.Bars) != 2 {
		t.Fatalf("mid gap %+v", kept)
	}
}

func TestPricePathATRStopsAtStar(t *testing.T) {
	k15 := pricePathBars(10, 20)
	star := pricePathStar(t, k15, 6, nodes.WozduhXoverSideUp)
	got, err := BuildPricePath(star, k15, 2)
	if err != nil {
		t.Fatal(err)
	}
	prefix := k15[:7]
	high := make([]float64, len(prefix))
	low := make([]float64, len(prefix))
	cl := make([]float64, len(prefix))
	for i, k := range prefix {
		high[i], low[i], cl[i] = k.High, k.Low, k.Close
	}
	series, err := indicators.ATRSeries(indicators.CanonicalATRSpec(), high, low, cl)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ATROK || got.ATR != series[len(series)-1] {
		t.Fatalf("atr %v ok %v want %v", got.ATR, got.ATROK, series[len(series)-1])
	}
	later := append([]exchange.Kline(nil), k15...)
	later[7].High += 500
	again, err := BuildPricePath(star, later, 2)
	if err != nil {
		t.Fatal(err)
	}
	if again.ATR != got.ATR {
		t.Fatalf("future bar changed ATR %v -> %v", got.ATR, again.ATR)
	}
}

func TestPricePathATRAbsentWhenFlat(t *testing.T) {
	k15 := pricePathBars(5, 10)
	for i := range k15 {
		k15[i].Open, k15[i].High, k15[i].Low, k15[i].Close = 10, 10, 10, 10
	}
	star := pricePathStar(t, k15, 3, nodes.WozduhXoverSideUp)
	got, err := BuildPricePath(star, k15, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.ATROK || got.ATR != 0 {
		t.Fatalf("invented atr %+v", got.ATR)
	}
	if got.Bars[0].High != 10 {
		t.Fatal("raw high was rewritten")
	}
}

func TestPricePathDeterministicAndHasNoOutcomeFields(t *testing.T) {
	k15 := pricePathBars(9, 30)
	star := pricePathStar(t, k15, 3, nodes.WozduhXoverSideUp)
	a, err := BuildPricePath(star, k15, 4)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildPricePath(star, k15, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("paths differ")
	}
	kind := reflect.TypeOf(PricePath{})
	for _, name := range []string{"TP", "SL", "Horizon", "Outcome", "MFE", "MAE"} {
		if _, ok := kind.FieldByName(name); ok {
			t.Fatalf("field %s", name)
		}
	}
	short, err := BuildPricePath(star, k15, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(short.Bars) != 1 || short.Signal != a.Signal {
		t.Fatalf("read length changed the path identity %+v", short)
	}
}

func TestResolveMinuteTouchUsesMinuteOpen(t *testing.T) {
	parent := int64(1_700_000_000_000)
	parent -= parent % (15 * 60 * 1000)
	mins := make([]exchange.Kline, 15)
	for i := range mins {
		open := parent + int64(i)*60_000
		mins[i] = exchange.Kline{OpenTime: open, Open: 100, High: 101, Low: 99, Close: 100}
	}
	mins[3].High = 110
	got, err := ResolveMinuteTouch(mins, parent, nodes.WozduhXoverSideUp, 105, 90)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != MinuteFavorable || got.HitOpen != mins[3].OpenTime {
		t.Fatalf("hit %+v", got)
	}

	mins[3].Low = 80
	both, err := ResolveMinuteTouch(mins, parent, nodes.WozduhXoverSideUp, 105, 90)
	if err != nil {
		t.Fatal(err)
	}
	if both.State != MinuteFinerDual || both.HitOpen != 0 {
		t.Fatalf("dual %+v", both)
	}

	missing, err := ResolveMinuteTouch(nil, parent, nodes.WozduhXoverSideDown, 90, 110)
	if err != nil {
		t.Fatal(err)
	}
	if missing.State != MinuteFinerMissing {
		t.Fatalf("missing %+v", missing)
	}

	gapped := append([]exchange.Kline(nil), mins[:2]...)
	gapped = append(gapped, mins[3:]...)
	gap, err := ResolveMinuteTouch(gapped, parent, nodes.WozduhXoverSideUp, 105, 90)
	if err != nil {
		t.Fatal(err)
	}
	if gap.State != MinuteFinerGap {
		t.Fatalf("gap %+v", gap)
	}

	for i := range mins {
		mins[i].High, mins[i].Low = 101, 99
	}
	none, err := ResolveMinuteTouch(mins, parent, nodes.WozduhXoverSideUp, 105, 90)
	if err != nil {
		t.Fatal(err)
	}
	if none.State != MinuteFinerInconsistent || none.HitOpen != 0 {
		t.Fatalf("inconsistent %+v", none)
	}
}

func TestPricePathArchiveSmoke(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(root, "history.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("history.db not present")
	}
	now := time.Now().UnixMilli()
	k15, err := loadFuturesClosedForPath(dbPath, "15m", exchange.BinanceFuturesGenesisMs, now)
	if err != nil {
		t.Fatal(err)
	}
	k1h, err := loadFuturesClosedForPath(dbPath, "1h", exchange.BinanceFuturesGenesisMs, now)
	if err != nil {
		t.Fatal(err)
	}
	k4h, err := loadFuturesClosedForPath(dbPath, "4h", exchange.BinanceFuturesGenesisMs, now)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ExtractStarSnapshots(k15, k1h, k4h, defaultRSXSettings())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 8 {
		t.Fatalf("stars %d", len(rows))
	}
	sample := sampleStars(rows, 16)
	const readBars = 8
	first, err := buildSample(sample, k15, readBars)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildSample(sample, k15, readBars)
	if err != nil {
		t.Fatal(err)
	}
	if pathDigest(first) != pathDigest(second) {
		t.Fatal("smoke digest changed")
	}
	var fill, miss, filled, trunc, gap, atrOK int
	var atrMiss int64
	for _, p := range first {
		if p.FillOK {
			fill++
		} else {
			miss++
		}
		switch p.Complete {
		case PathFilled:
			filled++
		case PathTruncated:
			trunc++
		case PathPrimaryGap:
			gap++
		default:
			t.Fatalf("complete %q", p.Complete)
		}
		if p.ATROK {
			atrOK++
		} else if atrMiss == 0 {
			atrMiss = p.StarOpen
		}
		for _, b := range p.Bars {
			if b.OpenTime <= p.StarOpen {
				t.Fatalf("bar %d not after star %d", b.OpenTime, p.StarOpen)
			}
		}
	}
	t.Logf("stars=%d sampled=%d fill=%d miss=%d filled=%d trunc=%d gap=%d atr=%d atrMissOpen=%d digest=%s readBars=%d",
		len(rows), len(first), fill, miss, filled, trunc, gap, atrOK, atrMiss, pathDigest(first), readBars)
}

func buildSample(stars []StarSnapshot, k15 []exchange.Kline, readBars int) ([]PricePath, error) {
	out := make([]PricePath, len(stars))
	for i, star := range stars {
		p, err := BuildPricePath(star, k15, readBars)
		if err != nil {
			return nil, err
		}
		out[i] = p
	}
	return out, nil
}

func sampleStars(rows []StarSnapshot, n int) []StarSnapshot {
	if len(rows) <= n {
		return append([]StarSnapshot(nil), rows...)
	}
	out := make([]StarSnapshot, 0, n)
	for i := 0; i < n; i++ {
		idx := i * (len(rows) - 1) / (n - 1)
		out = append(out, rows[idx])
	}
	return out
}

func pathDigest(paths []PricePath) string {
	h := sha256.New()
	var buf [8]byte
	putI := func(v int64) {
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		_, _ = h.Write(buf[:])
	}
	putF := func(v float64) {
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
		_, _ = h.Write(buf[:])
	}
	for _, p := range paths {
		putI(p.StarOpen)
		putI(p.StarClose)
		_, _ = h.Write([]byte(p.Side))
		putF(p.Signal)
		putF(p.Fill)
		putF(p.Gap)
		putF(p.ATR)
		_, _ = h.Write([]byte(p.Complete))
		putI(int64(len(p.Bars)))
		for _, b := range p.Bars {
			putI(b.OpenTime)
			putF(b.Open)
			putF(b.High)
			putF(b.Low)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func loadFuturesClosedForPath(dbPath, interval string, start, now int64) ([]exchange.Kline, error) {
	db, err := data.OpenReadOnlyHistory(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	qrows, err := db.Query(`
SELECT open_time, open, high, low, close, volume, close_time
FROM historical_klines
WHERE symbol = ? AND interval = ? AND open_time >= ? AND open_time <= ?
ORDER BY open_time ASC`, "BTCUSDT", interval, start, now)
	if err != nil {
		return nil, err
	}
	defer qrows.Close()
	var out []exchange.Kline
	for qrows.Next() {
		var k exchange.Kline
		if err := qrows.Scan(&k.OpenTime, &k.Open, &k.High, &k.Low, &k.Close, &k.Volume, &k.CloseTime); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	if err := qrows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, os.ErrNotExist
	}
	last := out[len(out)-1]
	if data.IsFormingCloseTime(last.CloseTime, now) {
		out = out[:len(out)-1]
	}
	return out, nil
}

func pricePathBars(n int, base float64) []exchange.Kline {
	const step int64 = 15 * 60 * 1000
	start := int64(1_700_000_000_000)
	start -= start % step
	out := make([]exchange.Kline, n)
	for i := range out {
		px := base + float64(i)
		open := start + int64(i)*step
		ct, err := data.BarCloseTimeMs(open, "15m")
		if err != nil {
			panic(err)
		}
		out[i] = exchange.Kline{
			OpenTime: open, CloseTime: ct,
			Open: px, High: px + 2, Low: px - 1, Close: px + 0.5, Volume: 1,
		}
	}
	return out
}

func pricePathStar(t *testing.T, k15 []exchange.Kline, idx int, side string) StarSnapshot {
	t.Helper()
	k := k15[idx]
	return StarSnapshot{
		Side: side, AnchorAt: k.OpenTime, ConfirmedAt: k.OpenTime,
		M15: StarTF{Present: true, OpenTime: k.OpenTime, CloseTime: k.CloseTime},
	}
}
