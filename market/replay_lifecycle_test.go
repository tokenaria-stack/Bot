package market

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"trading_bot/core"
	"trading_bot/data"
	"trading_bot/exchange"
)

func synthClosedPrefix(n int, step, nowMs int64) []exchange.Kline {
	closed := make([]exchange.Kline, n)
	base := ((nowMs - int64(n+2)*step) / step) * step
	for i := 0; i < n; i++ {
		ot := base + int64(i)*step
		p := 100 + math.Sin(float64(i)*0.2)*5
		closed[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime: ot, CloseTime: ot + step - 1,
			Open: p, High: p + 1.5, Low: p - 1.5, Close: p + 0.3, Volume: 10,
		})
	}
	return closed
}

func synthFormingTip(afterClosed exchange.Kline, step, nowMs int64) exchange.Kline {
	ot := afterClosed.OpenTime + step
	forming := exchange.NormalizeKline(exchange.Kline{
		OpenTime: ot, CloseTime: nowMs + 40_000,
		Open: 110, High: 116, Low: 108, Close: 114.5, Volume: 55,
	})
	if !data.IsFormingCloseTime(forming.CloseTime, nowMs) {
		forming.CloseTime = nowMs + 40_000
	}
	return forming
}

func withTempRSX(t *testing.T, settings RSXSettings) {
	t.Helper()
	ResetRSXSettings()
	SetRSXSettingsPath(filepath.Join(t.TempDir(), "rsx.json"))
	t.Cleanup(func() {
		ResetRSXSettings()
		SetRSXSettingsPath("")
	})
	_ = ApplyRSXSettings(settings)
}

func withEngineMode(t *testing.T, mode EngineMode) {
	t.Helper()
	prev := GetEngineMode()
	SetEngineMode(mode)
	t.Cleanup(func() { SetEngineMode(prev) })
}

func TestSplitLiveTail_CalendarNotLastBarHeuristic(t *testing.T) {
	now := int64(1_700_000_000_000)
	step := int64(60_000)
	closedA := exchange.NormalizeKline(exchange.Kline{
		OpenTime: now - 2*step, CloseTime: now - step - 1,
		Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 1,
	})
	closedB := exchange.NormalizeKline(exchange.Kline{
		OpenTime: now - step, CloseTime: now - 1,
		Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 1,
	})
	forming := exchange.NormalizeKline(exchange.Kline{
		OpenTime: now, CloseTime: now + step - 1,
		Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 1,
	})

	c, f := splitLiveTail([]exchange.Kline{closedA, closedB}, now)
	if f != nil || len(c) != 2 {
		t.Fatalf("all-closed: closed=%d forming=%v", len(c), f != nil)
	}

	c, f = splitLiveTail([]exchange.Kline{closedA, closedB, forming}, now)
	if f == nil || f.OpenTime != forming.OpenTime || len(c) != 2 {
		t.Fatalf("with forming: closed=%d formingOT=%v", len(c), f)
	}

	c, f = splitLiveTail([]exchange.Kline{forming}, now)
	if len(c) != 0 || f == nil || f.OpenTime != forming.OpenTime {
		t.Fatalf("sole forming: closed=%d forming=%v", len(c), f != nil)
	}

	unknown := exchange.NormalizeKline(exchange.Kline{
		OpenTime: now, CloseTime: 0, Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 1,
	})
	c, f = splitLiveTail([]exchange.Kline{closedA, unknown}, now)
	if f != nil || len(c) != 2 {
		t.Fatalf("CloseTime=0 must stay closed: closed=%d forming=%v", len(c), f != nil)
	}
}

// ADR-016: IndicatorReplay must leave forming tip uncommitted and match live Cur.
func TestReplayLifecycle_IndicatorReplayPreservesFormingCur(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	withTempRSX(t, RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})

	step := int64(60_000)
	nowMs := time.Now().UnixMilli()
	closed := synthClosedPrefix(80, step, nowMs)
	forming := synthFormingTip(closed[len(closed)-1], step, nowMs)

	frame := NewFrame(append(append([]exchange.Kline{}, closed...), forming), "1m", ChaosConfig{})
	// Ensure live semantics even if NewFrame path changes: forming as open.
	frame.UpdateKlineTick(forming, false)

	prev := GetRSXSettings()
	next := NormalizeRSXSettings(mergeRSXSettings(prev, RSXSettings{Length: 21}))
	_ = ApplyRSXSettings(next)
	frame.UpdateRSXScanConfig(prev, next)

	postCur := frame.DAGTickFrame().Get(core.SlotJurikRSX)
	if frame.lastCommittedOpenTime == forming.OpenTime {
		t.Fatalf("forming open must not be committed after replay: lastCommitted=%d forming=%d",
			frame.lastCommittedOpenTime, forming.OpenTime)
	}
	if len(closed) > 0 && frame.lastCommittedOpenTime != closed[len(closed)-1].OpenTime {
		t.Fatalf("lastCommitted want last closed %d got %d",
			closed[len(closed)-1].OpenTime, frame.lastCommittedOpenTime)
	}

	ref := NewFrame(append([]exchange.Kline{}, closed...), "1m", ChaosConfig{})
	ref.UpdateKlineTick(forming, false)
	want := ref.DAGTickFrame().Get(core.SlotJurikRSX)
	if math.Abs(postCur-want) > 1e-9 {
		t.Fatalf("post-IndicatorReplay Cur=%.10f want live forming=%.10f |Δ|=%.6e",
			postCur, want, math.Abs(postCur-want))
	}
}

// ADR-016: first WS tick with same OHLC after settings replay must not move Jurik.
func TestReplayLifecycle_FirstWSFormingTickIdempotent(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	withTempRSX(t, RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})

	step := int64(60_000)
	nowMs := time.Now().UnixMilli()
	closed := synthClosedPrefix(60, step, nowMs)
	forming := synthFormingTip(closed[len(closed)-1], step, nowMs)

	frame := NewFrame(append(append([]exchange.Kline{}, closed...), forming), "1m", ChaosConfig{})
	frame.UpdateKlineTick(forming, false)

	prev := GetRSXSettings()
	next := NormalizeRSXSettings(mergeRSXSettings(prev, RSXSettings{Source: "close"}))
	_ = ApplyRSXSettings(next)
	frame.UpdateRSXScanConfig(prev, next)

	afterReplay := frame.DAGTickFrame().Get(core.SlotJurikRSX)
	frame.UpdateKlineTick(forming, false)
	afterWS := frame.DAGTickFrame().Get(core.SlotJurikRSX)
	if math.Abs(afterReplay-afterWS) > 1e-9 {
		t.Fatalf("WS forming tick jumped Cur: replay=%.10f ws=%.10f |Δ|=%.6e",
			afterReplay, afterWS, math.Abs(afterReplay-afterWS))
	}
	if frame.lastCommittedOpenTime == forming.OpenTime {
		t.Fatal("forming tip committed after WS tick")
	}
}

// ADR-016: all-closed Frame replay still commits the last closed bar (unchanged contract).
func TestReplayLifecycle_ClosedOnlyUnchanged(t *testing.T) {
	withEngineMode(t, EngineModeChartOnly)
	withTempRSX(t, RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})

	step := int64(60_000)
	nowMs := time.Now().UnixMilli()
	closed := synthClosedPrefix(40, step, nowMs)

	frame := NewFrame(append([]exchange.Kline{}, closed...), "1m", ChaosConfig{})
	before := frame.DAGTickFrame().Get(core.SlotJurikRSX)
	beforeCommit := frame.lastCommittedOpenTime

	prev := GetRSXSettings()
	next := NormalizeRSXSettings(mergeRSXSettings(prev, RSXSettings{Length: 21}))
	_ = ApplyRSXSettings(next)
	frame.UpdateRSXScanConfig(prev, next)

	ref := NewFrame(append([]exchange.Kline{}, closed...), "1m", ChaosConfig{})
	want := ref.DAGTickFrame().Get(core.SlotJurikRSX)
	got := frame.DAGTickFrame().Get(core.SlotJurikRSX)
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("closed-only replay Cur=%.10f want=%.10f", got, want)
	}
	if frame.lastCommittedOpenTime != closed[len(closed)-1].OpenTime {
		t.Fatalf("closed-only commit want %d got %d (before=%d beforeCur=%.6f)",
			closed[len(closed)-1].OpenTime, frame.lastCommittedOpenTime, beforeCommit, before)
	}
}

func TestReplayLifecycle_LiveModeFormingNotCommitted(t *testing.T) {
	withEngineMode(t, EngineModeLive)
	withTempRSX(t, RSXSettings{Length: 14, SignalLength: 9, Source: "hlc3"})

	step := int64(60_000)
	nowMs := time.Now().UnixMilli()
	closed := synthClosedPrefix(50, step, nowMs)
	forming := synthFormingTip(closed[len(closed)-1], step, nowMs)

	frame := NewFrame(append(append([]exchange.Kline{}, closed...), forming), "1m", ChaosConfig{})
	frame.UpdateKlineTick(forming, false)

	prev := GetRSXSettings()
	next := NormalizeRSXSettings(mergeRSXSettings(prev, RSXSettings{Length: 18}))
	_ = ApplyRSXSettings(next)
	frame.UpdateRSXScanConfig(prev, next)

	if frame.lastCommittedOpenTime == forming.OpenTime {
		t.Fatal("Live mode: forming committed after IndicatorReplay")
	}
	afterReplay := frame.DAGTickFrame().Get(core.SlotJurikRSX)
	frame.UpdateKlineTick(forming, false)
	afterWS := frame.DAGTickFrame().Get(core.SlotJurikRSX)
	if math.Abs(afterReplay-afterWS) > 1e-9 {
		t.Fatalf("Live mode WS jump: %.10f → %.10f", afterReplay, afterWS)
	}
}
