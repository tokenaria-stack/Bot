package exchange

import (
	"encoding/json"
	"testing"

	binance "github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
)

func TestVolumeClassify_TotalBaseNotTaker(t *testing.T) {
	t.Parallel()
	auth := VolumeAuthority{Base: 389.735, TakerBuyBase: 177.127, Quote: 3.1e7, TakerBuyQuote: 1.4e7}
	if ClassifyStoredVolume(389.735, auth) != VolumeClassTotalBase {
		t.Fatal("v")
	}
	if ClassifyStoredVolume(177.127, auth) != VolumeClassTakerBuyBase {
		t.Fatal("V")
	}
	if ClassifyStoredVolume(3.1e7, auth) != VolumeClassTotalQuote {
		t.Fatal("q")
	}
	if ClassifyStoredVolume(1.4e7, auth) != VolumeClassTakerBuyQuote {
		t.Fatal("Q")
	}
}

func TestCandleFromFuturesRESTKline_VolumeIsVNotTaker(t *testing.T) {
	t.Parallel()
	k := &futures.Kline{
		OpenTime: 1, CloseTime: 2,
		Open: "1", High: "2", Low: "0.5", Close: "1.5",
		Volume:                   "100.5",
		QuoteAssetVolume:         "99999",
		TakerBuyBaseAssetVolume:  "12.25",
		TakerBuyQuoteAssetVolume: "8888",
	}
	c, err := CandleFromFuturesRESTKline(k)
	if err != nil {
		t.Fatal(err)
	}
	if c.Volume != 100.5 {
		t.Fatalf("Volume=%v want 100.5 (index 5 / v)", c.Volume)
	}
	if c.Volume == 12.25 || c.Volume == 99999 || c.Volume == 8888 {
		t.Fatal("Volume must not be V/q/Q")
	}
}

func TestCandleFromSpotRESTKline_VolumeIsTotalBase(t *testing.T) {
	t.Parallel()
	k := &binance.Kline{
		OpenTime: 1, CloseTime: 2,
		Open: "1", High: "2", Low: "0.5", Close: "1.5",
		Volume:                   "46.66",
		QuoteAssetVolume:         "3727835",
		TakerBuyBaseAssetVolume:  "23.41",
		TakerBuyQuoteAssetVolume: "1800000",
	}
	c, err := CandleFromSpotRESTKline(k)
	if err != nil {
		t.Fatal(err)
	}
	if c.Volume != 46.66 {
		t.Fatalf("spot Volume=%v want 46.66", c.Volume)
	}
}

func TestParseFuturesWsKlineJSON_VolumeIsLowercaseV(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"e":"kline","E":1700000000000,"s":"BTCUSDT",
		"k":{"t":1700000000000,"T":1700000899999,"s":"BTCUSDT","i":"15m",
		"o":"1","c":"1","h":"1","l":"1",
		"v":"100.5","V":"12.25","q":"99999","Q":"8888","x":true}
	}`)
	tick, err := ParseFuturesWsKlineJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if tick.Kline.Volume != 100.5 {
		t.Fatalf("Volume=%v want v=100.5", tick.Kline.Volume)
	}
	if tick.Kline.OpenTime != 1700000000000 || tick.Kline.CloseTime != 1700000899999 {
		t.Fatalf("t/T mixed: %+v", tick.Kline)
	}
	if !tick.IsClosed || tick.Timeframe != "15m" {
		t.Fatalf("tick %+v", tick)
	}
}

func TestEncodingJSON_StructTagVIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	// Documents why production must not Unmarshal into json:"v" when Binance also sends "V".
	raw := []byte(`{"v":"100.5","V":"12.25"}`)
	var hit struct {
		Volume flexString `json:"v"`
	}
	if err := json.Unmarshal(raw, &hit); err != nil {
		t.Fatal(err)
	}
	got, err := hit.Volume.Float64()
	if err != nil {
		t.Fatal(err)
	}
	if got == 100.5 {
		t.Fatal("expected encoding/json to let V overwrite v; if this fails, Go json became case-sensitive")
	}
	if got != 12.25 {
		t.Fatalf("got %v want 12.25 (V last-wins)", got)
	}
}

func TestCandleFromVisionCSV_Column5(t *testing.T) {
	t.Parallel()
	row := []string{
		"1788655500000", "79838.7", "79897.4", "79823.8", "79832.8",
		"389.735", "1788656399999", "31123774.5", "10655", "177.127", "14145520.9",
	}
	c, err := CandleFromVisionCSV(row)
	if err != nil {
		t.Fatal(err)
	}
	if c.Volume != 389.735 {
		t.Fatalf("Volume=%v want col5 389.735", c.Volume)
	}
	if c.OpenTime != 1788655500000 {
		t.Fatal(c.OpenTime)
	}
}

func TestParseAggTradeJSON_DropsMakerFlag(t *testing.T) {
	t.Parallel()
	buy := []byte(`{"e":"aggTrade","E":1,"s":"BTCUSDT","a":1,"p":"100","q":"1.5","T":1700000000000,"m":false}`)
	sell := []byte(`{"e":"aggTrade","E":1,"s":"BTCUSDT","a":2,"p":"100","q":"1.5","T":1700000000000,"m":true}`)
	a, err := ParseAggTradeJSON(buy)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseAggTradeJSON(sell)
	if err != nil {
		t.Fatal(err)
	}
	if a.Qty != b.Qty || a.Qty != 1.5 {
		t.Fatalf("qty changed with m: %+v %+v", a, b)
	}
}

func TestSecondBarBuilder_SumsAllQtyRegardlessOfWouldBeSide(t *testing.T) {
	t.Parallel()
	takerBuy, err := ParseAggTradeJSON([]byte(`{"p":"100","q":"1","T":1700000010010,"m":false}`))
	if err != nil {
		t.Fatal(err)
	}
	maker, err := ParseAggTradeJSON([]byte(`{"p":"101","q":"10","T":1700000010020,"m":true}`))
	if err != nil {
		t.Fatal(err)
	}
	b := NewSecondBarBuilder()
	_, f, _, ok := b.OnAggTrade(takerBuy)
	if !ok || f.Volume != 1 {
		t.Fatalf("first %+v ok=%v", f, ok)
	}
	_, f, _, ok = b.OnAggTrade(maker)
	if !ok || f.Volume != 11 {
		t.Fatalf("want sum of ALL qty=11 got %v (not taker-buy-only 1)", f.Volume)
	}
}

func TestAssembleChildOHLCV_SumsSameSemanticVolume(t *testing.T) {
	t.Parallel()
	parents := []Kline{
		{OpenTime: 0, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 3},
		{OpenTime: 60_000, Open: 1.5, High: 3, Low: 1, Close: 2, Volume: 4},
	}
	got, err := AssembleChildOHLCV(0, "2m", parents)
	if err != nil {
		t.Fatal(err)
	}
	if got.Volume != 7 {
		t.Fatalf("derived Volume=%v want 7", got.Volume)
	}
}
