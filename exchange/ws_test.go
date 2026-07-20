package exchange

import (
	"encoding/json"
	"sync/atomic"
	"testing"
)

func TestWsClientLifecycleCallbacks(t *testing.T) {
	c := NewWsClient("BTCUSDT", nil)
	var reconnects, disconnects atomic.Int32
	c.SetOnReconnect(func() { reconnects.Add(1) })
	c.SetOnDisconnect(func() { disconnects.Add(1) })

	c.fireReconnect()
	c.fireDisconnect()
	if reconnects.Load() != 1 || disconnects.Load() != 1 {
		t.Fatalf("got reconnects=%d disconnects=%d want 1/1", reconnects.Load(), disconnects.Load())
	}

	// Nil hooks are no-ops (safe before wiring).
	c2 := NewWsClient("ETHUSDT", nil)
	c2.fireReconnect()
	c2.fireDisconnect()
}

func TestHandleAggTradePayload(t *testing.T) {
	raw := []byte(`{
		"e":"aggTrade","E":1700000000000,"s":"BTCUSDT","a":123,
		"p":"65000.10","q":"0.015","f":100,"l":101,"T":1700000000123,"m":false
	}`)

	var event wsAggTradeEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if event.Price != "65000.10" || event.TradeTime != 1700000000123 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestHandleKlinePayload(t *testing.T) {
	raw := []byte(`{
		"e":"kline","E":1700000000000,"s":"BTCUSDT",
		"k":{"t":1700000000000,"T":1700000060000,"s":"BTCUSDT","i":"1m",
		"o":"65000","c":"65100","h":"65200","l":"64900","v":"12.3","x":false}
	}`)

	var event wsKlinePayload
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if event.Kline.Interval != "1m" || event.Kline.Close != "65100" {
		t.Fatalf("unexpected kline: %+v", event.Kline)
	}
}

func TestHandleKlinePayloadNumericOHLC(t *testing.T) {
	raw := []byte(`{
		"e":"kline","E":1700000000000,"s":"BTCUSDT",
		"k":{"t":1700000000000,"T":1700000060000,"s":"BTCUSDT","i":"15m",
		"o":65000.1,"c":65100,"h":65200.5,"l":64900,"v":12.3,"x":false}
	}`)

	var event wsKlinePayload
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("unmarshal numeric OHLC: %v", err)
	}
	low, err := event.Kline.Low.Float64()
	if err != nil || low != 64900 {
		t.Fatalf("low = %v err = %v", low, err)
	}
	closePrice, err := event.Kline.Close.Float64()
	if err != nil || closePrice != 65100 {
		t.Fatalf("close = %v", closePrice)
	}
}
