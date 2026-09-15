package exchange

import "math"

// VOLUME-INGEST-1 — semantic law for candle volume.
//
// Native / exchange klines (futures REST, futures WS, Vision, spot Vision/REST):
//
//	Kline.Volume ≡ BaseVolume ≡ Binance total BASE asset volume
//	            ≡ REST array index 5 ≡ WS JSON "v" ≡ Vision CSV column 5
//
// For BTCUSDT the unit is BTC. It must never mean taker-buy base "V", quote "q",
// or taker-buy quote "Q". Taker-buy base is a reserved future semantic
// (TakerBuyBaseVolume ≡ Binance "V") and is not stored on Kline in this chapter.
//
// Micro / 1s (SecondBarBuilder → micro_klines):
//
//	Kline.Volume ≡ MicroTradeBaseVolume ≡ sum of aggTrade Qty for EVERY trade
//	in that second. Side (IsBuyerMaker / "m") is dropped before the builder.
//	This is NOT Binance kline "V". It is also not claimed equal to native
//	exchange kline "v" unless a separate source-parity test proves it.
//
// Sparse seconds (5s–45s) and derived minute/hour views sum child Volume of the
// SAME semantic stream (DERIVED_SAME_SEMANTIC).
//
// Ingress/SQLite Volume=MAX is legal only when both values are already BaseVolume
// (or both MicroTradeBaseVolume on the micro path). MAX is not v/V coercion.

// VolumeProducerClass is the VOLUME-INGEST-1 producer inventory class.
type VolumeProducerClass string

const (
	VolumeNativeBase     VolumeProducerClass = "NATIVE_BASE_VOLUME"
	VolumeMicroTradeBase VolumeProducerClass = "MICRO_TRADE_BASE_VOLUME"
	VolumeDerivedSame    VolumeProducerClass = "DERIVED_SAME_SEMANTIC"
	VolumeUnknown        VolumeProducerClass = "UNKNOWN"
)

// VolumeClass is a census label for stored Volume vs an authoritative kline.
type VolumeClass string

const (
	VolumeClassTotalBase     VolumeClass = "TOTAL_BASE"
	VolumeClassTakerBuyBase  VolumeClass = "TAKER_BUY_BASE"
	VolumeClassTotalQuote    VolumeClass = "TOTAL_QUOTE"
	VolumeClassTakerBuyQuote VolumeClass = "TAKER_BUY_QUOTE"
	VolumeClassZeroMissing   VolumeClass = "ZERO_OR_MISSING"
	VolumeClassMatchNone     VolumeClass = "MATCH_NONE"
)

// VolumeAuthority is Binance kline volume fields for one open time. Not a Kline.
type VolumeAuthority struct {
	OpenTime      int64
	Open          float64
	High          float64
	Low           float64
	Close         float64
	Base          float64 // v / index 5
	TakerBuyBase  float64 // V / index 9
	Quote         float64 // q / index 7
	TakerBuyQuote float64 // Q / index 10
	Trades        int64   // REST index 8 / Vision col 8 (forensic only)
}

// VolumeFloatEqual is the census equality: float64 as persisted (SQLite REAL /
// strconv.ParseFloat). It does not treat 1% relative error as a match.
func VolumeFloatEqual(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}
	if a == b {
		return true
	}
	d := math.Abs(a - b)
	scale := math.Max(math.Abs(a), math.Abs(b))
	if scale == 0 {
		return d == 0
	}
	// 1e-9 relative, plus 1e-10 abs for tiny BTC fractions — not a 0.1% hide.
	return d <= 1e-9*scale || d <= 1e-10
}

// ClassifyStoredVolume compares one archived Volume to an authoritative kline.
func ClassifyStoredVolume(stored float64, auth VolumeAuthority) VolumeClass {
	if VolumeFloatEqual(stored, 0) && !VolumeFloatEqual(auth.Base, 0) {
		return VolumeClassZeroMissing
	}
	if VolumeFloatEqual(stored, auth.Base) {
		return VolumeClassTotalBase
	}
	if VolumeFloatEqual(stored, auth.TakerBuyBase) {
		return VolumeClassTakerBuyBase
	}
	if VolumeFloatEqual(stored, auth.Quote) {
		return VolumeClassTotalQuote
	}
	if VolumeFloatEqual(stored, auth.TakerBuyQuote) {
		return VolumeClassTakerBuyQuote
	}
	if VolumeFloatEqual(stored, 0) && VolumeFloatEqual(auth.Base, 0) {
		return VolumeClassZeroMissing
	}
	return VolumeClassMatchNone
}
