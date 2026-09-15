package exchange

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	binance "github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
)

// CandleFromFuturesRESTKline maps go-binance futures REST kline Volume (index 5 / v).
func CandleFromFuturesRESTKline(k *futures.Kline) (Candle, error) {
	if k == nil {
		return Candle{}, fmt.Errorf("nil futures kline")
	}
	return candleFromFuturesKline(k)
}

// CandleFromSpotRESTKline maps go-binance spot REST kline Volume (index 5 / v).
func CandleFromSpotRESTKline(k *binance.Kline) (Candle, error) {
	if k == nil {
		return Candle{}, fmt.Errorf("nil spot kline")
	}
	open, err := strconv.ParseFloat(k.Open, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse open: %w", err)
	}
	high, err := strconv.ParseFloat(k.High, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse high: %w", err)
	}
	low, err := strconv.ParseFloat(k.Low, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse low: %w", err)
	}
	closePrice, err := strconv.ParseFloat(k.Close, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse close: %w", err)
	}
	volume, err := strconv.ParseFloat(k.Volume, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse volume: %w", err)
	}
	return Candle{
		OpenTime:  k.OpenTime,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closePrice,
		Volume:    volume,
		CloseTime: k.CloseTime,
	}, nil
}

// CandleFromVisionCSV maps Binance Vision kline CSV: col0 open_time … col5 volume … col6 close_time.
func CandleFromVisionCSV(record []string) (Candle, error) {
	if len(record) < 7 {
		return Candle{}, fmt.Errorf("vision csv: need 7 columns, got %d", len(record))
	}
	openTime, err := strconv.ParseInt(strings.TrimSpace(record[0]), 10, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse open time %q: %w", record[0], err)
	}
	open, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse open: %w", err)
	}
	high, err := strconv.ParseFloat(strings.TrimSpace(record[2]), 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse high: %w", err)
	}
	low, err := strconv.ParseFloat(strings.TrimSpace(record[3]), 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse low: %w", err)
	}
	closePrice, err := strconv.ParseFloat(strings.TrimSpace(record[4]), 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse close: %w", err)
	}
	volume, err := strconv.ParseFloat(strings.TrimSpace(record[5]), 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse volume: %w", err)
	}
	closeTime, err := strconv.ParseInt(strings.TrimSpace(record[6]), 10, 64)
	if err != nil {
		return Candle{}, fmt.Errorf("parse close time: %w", err)
	}
	return Candle{
		OpenTime:  openTime,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closePrice,
		Volume:    volume,
		CloseTime: closeTime,
	}, nil
}

func jsonMapString(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func jsonField(m map[string]json.RawMessage, key string) (json.RawMessage, error) {
	v, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("missing %q", key)
	}
	return v, nil
}

func jsonFieldInt64(m map[string]json.RawMessage, key string) (int64, error) {
	raw, err := jsonField(m, key)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		var s string
		if err2 := json.Unmarshal(raw, &s); err2 != nil {
			return 0, err
		}
		return strconv.ParseInt(s, 10, 64)
	}
	return n, nil
}

func jsonFieldFlex(m map[string]json.RawMessage, key string) (float64, error) {
	raw, err := jsonField(m, key)
	if err != nil {
		return 0, err
	}
	var f flexString
	if err := f.UnmarshalJSON(raw); err != nil {
		return 0, err
	}
	return f.Float64()
}

func jsonFieldString(m map[string]json.RawMessage, key string) (string, error) {
	raw, err := jsonField(m, key)
	if err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func jsonFieldBool(m map[string]json.RawMessage, key string) (bool, error) {
	raw, err := jsonField(m, key)
	if err != nil {
		return false, err
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, err
	}
	return b, nil
}

// ParseFuturesWsKlineJSON maps combined-stream kline JSON.
// Keys are case-sensitive: Volume is "v" only, never "V" (Go encoding/json would
// otherwise treat v/V as the same field and let the later key win).
func ParseFuturesWsKlineJSON(raw []byte) (tick WsTick, err error) {
	root, err := jsonMapString(raw)
	if err != nil {
		return WsTick{}, err
	}
	kraw, err := jsonField(root, "k")
	if err != nil {
		return WsTick{}, err
	}
	k, err := jsonMapString(kraw)
	if err != nil {
		return WsTick{}, err
	}
	interval, err := jsonFieldString(k, "i")
	if err != nil || interval == "" {
		return WsTick{}, fmt.Errorf("ws kline: interval")
	}
	start, err := jsonFieldInt64(k, "t")
	if err != nil {
		return WsTick{}, fmt.Errorf("ws kline t: %w", err)
	}
	end, err := jsonFieldInt64(k, "T")
	if err != nil {
		return WsTick{}, fmt.Errorf("ws kline T: %w", err)
	}
	open, err := jsonFieldFlex(k, "o")
	if err != nil {
		return WsTick{}, fmt.Errorf("ws kline o: %w", err)
	}
	high, err := jsonFieldFlex(k, "h")
	if err != nil {
		return WsTick{}, fmt.Errorf("ws kline h: %w", err)
	}
	low, err := jsonFieldFlex(k, "l")
	if err != nil {
		return WsTick{}, fmt.Errorf("ws kline l: %w", err)
	}
	closePrice, err := jsonFieldFlex(k, "c")
	if err != nil {
		return WsTick{}, fmt.Errorf("ws kline c: %w", err)
	}
	volume, err := jsonFieldFlex(k, "v")
	if err != nil {
		return WsTick{}, fmt.Errorf("ws kline v: %w", err)
	}
	closed, err := jsonFieldBool(k, "x")
	if err != nil {
		return WsTick{}, fmt.Errorf("ws kline x: %w", err)
	}
	return WsTick{
		Timeframe: interval,
		IsClosed:  closed,
		Kline:     klineFromBinanceMs(start, end, open, high, low, closePrice, volume),
	}, nil
}

// ParseAggTradeJSON maps aggTrade JSON to exchange.AggTrade (qty only; "m" is not retained).
func ParseAggTradeJSON(raw []byte) (AggTrade, error) {
	var event wsAggTradeEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return AggTrade{}, err
	}
	price, err := strconv.ParseFloat(event.Price, 64)
	if err != nil || price <= 0 {
		return AggTrade{}, fmt.Errorf("aggTrade price")
	}
	qty, err := strconv.ParseFloat(event.Quantity, 64)
	if err != nil || qty <= 0 {
		return AggTrade{}, fmt.Errorf("aggTrade qty")
	}
	if event.TradeTime < 0 {
		return AggTrade{}, fmt.Errorf("aggTrade time")
	}
	return AggTrade{TimeMs: event.TradeTime, Price: price, Qty: qty}, nil
}

// VolumeAuthorityFromFuturesRESTKline keeps v/V/q/Q for census. It does not write Kline.
func VolumeAuthorityFromFuturesRESTKline(k *futures.Kline) (VolumeAuthority, error) {
	if k == nil {
		return VolumeAuthority{}, fmt.Errorf("nil kline")
	}
	base, err := strconv.ParseFloat(k.Volume, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	tb, err := strconv.ParseFloat(k.TakerBuyBaseAssetVolume, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	q, err := strconv.ParseFloat(k.QuoteAssetVolume, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	tq, err := strconv.ParseFloat(k.TakerBuyQuoteAssetVolume, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	open, err := strconv.ParseFloat(k.Open, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	high, err := strconv.ParseFloat(k.High, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	low, err := strconv.ParseFloat(k.Low, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	cls, err := strconv.ParseFloat(k.Close, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	return VolumeAuthority{
		OpenTime: k.OpenTime, Open: open, High: high, Low: low, Close: cls,
		Base: base, TakerBuyBase: tb, Quote: q, TakerBuyQuote: tq, Trades: k.TradeNum,
	}, nil
}
func VolumeAuthorityFromSpotRESTKline(k *binance.Kline) (VolumeAuthority, error) {
	if k == nil {
		return VolumeAuthority{}, fmt.Errorf("nil kline")
	}
	base, err := strconv.ParseFloat(k.Volume, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	tb, err := strconv.ParseFloat(k.TakerBuyBaseAssetVolume, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	q, err := strconv.ParseFloat(k.QuoteAssetVolume, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	tq, err := strconv.ParseFloat(k.TakerBuyQuoteAssetVolume, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	open, err := strconv.ParseFloat(k.Open, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	high, err := strconv.ParseFloat(k.High, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	low, err := strconv.ParseFloat(k.Low, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	cls, err := strconv.ParseFloat(k.Close, 64)
	if err != nil {
		return VolumeAuthority{}, err
	}
	return VolumeAuthority{
		OpenTime: k.OpenTime, Open: open, High: high, Low: low, Close: cls,
		Base: base, TakerBuyBase: tb, Quote: q, TakerBuyQuote: tq,
	}, nil
}

// VolumeAuthorityFromVisionCandle maps Vision CSV (col5 = v) for recovery identity.
func VolumeAuthorityFromVisionCandle(c Candle) VolumeAuthority {
	return VolumeAuthority{
		OpenTime: c.OpenTime, Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Base: c.Volume,
	}
}

// VolumeAuthorityFromVisionCSV maps Vision kline CSV including quote (col7) and
// trade count (col8) for forensic aggregation. It does not write Kline.
func VolumeAuthorityFromVisionCSV(record []string) (VolumeAuthority, error) {
	c, err := CandleFromVisionCSV(record)
	if err != nil {
		return VolumeAuthority{}, err
	}
	a := VolumeAuthorityFromVisionCandle(c)
	if len(record) > 7 {
		q, err := strconv.ParseFloat(strings.TrimSpace(record[7]), 64)
		if err == nil {
			a.Quote = q
		}
	}
	if len(record) > 8 {
		n, err := strconv.ParseInt(strings.TrimSpace(record[8]), 10, 64)
		if err == nil {
			a.Trades = n
		}
	}
	if len(record) > 9 {
		tb, err := strconv.ParseFloat(strings.TrimSpace(record[9]), 64)
		if err == nil {
			a.TakerBuyBase = tb
		}
	}
	return a, nil
}
