package exchange

import "trading_bot/data"

const SecondTF = "1s"

// AggTrade is one USD-M aggTrade event. Not retained after the 1s builder consumes it.
type AggTrade struct {
	TimeMs int64
	Price  float64
	Qty    float64
}

// SecondBarBuilder folds aggTrade events into 1s OHLCV. State is the current
// forming bar only — no raw-event list.
type SecondBarBuilder struct {
	cur   Kline
	has   bool
	floor int64 // last hydrated/persisted closed open; trades in that second are dropped
}

func NewSecondBarBuilder() *SecondBarBuilder {
	return &SecondBarBuilder{}
}

// SeedClosedFloor records the last closed 1s open after boot hydrate.
// Trades whose bucket is at or before floor do not reopen that bar.
func (b *SecondBarBuilder) SeedClosedFloor(openMs int64) {
	if b == nil || openMs <= 0 {
		return
	}
	if openMs > b.floor {
		b.floor = openMs
	}
}

func newSecondBar(open int64, t AggTrade) Kline {
	ct, err := data.BarCloseTimeMs(open, SecondTF)
	if err != nil {
		ct = open + 999
	}
	p := t.Price
	return Kline{
		OpenTime:  open,
		CloseTime: ct,
		Open:      p,
		High:      p,
		Low:       p,
		Close:     p,
		Volume:    t.Qty,
	}
}

func applyAggTradeToBar(k *Kline, t AggTrade) {
	if t.Price > k.High {
		k.High = t.Price
	}
	if t.Price < k.Low {
		k.Low = t.Price
	}
	k.Close = t.Price
	k.Volume += t.Qty
}

// OnAggTrade updates the forming 1s bar. Bucket identity is trade time T.
// A later second closes the previous bar exactly once. Late T is dropped.
func (b *SecondBarBuilder) OnAggTrade(t AggTrade) (closed Kline, forming Kline, didClose, hasForming bool) {
	if b == nil || t.TimeMs < 0 || t.Price <= 0 || t.Qty <= 0 {
		return
	}
	open, err := data.CurrentBarOpen(t.TimeMs, SecondTF)
	if err != nil {
		return
	}
	if b.has && t.TimeMs < b.cur.OpenTime {
		return
	}
	if b.floor > 0 && open <= b.floor {
		return
	}
	if !b.has {
		b.cur = newSecondBar(open, t)
		b.has = true
		return Kline{}, b.cur, false, true
	}
	if open == b.cur.OpenTime {
		applyAggTradeToBar(&b.cur, t)
		return Kline{}, b.cur, false, true
	}
	closed = b.cur
	b.cur = newSecondBar(open, t)
	return closed, b.cur, true, true
}
