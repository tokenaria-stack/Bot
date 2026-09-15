package exchange

import (
	"context"
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

func TestHandleAggTradeSendsTradeTimeNotEventTime(t *testing.T) {
	c := NewWsClient("BTCUSDT", nil)
	raw := []byte(`{
		"e":"aggTrade","E":1700000000999,"s":"BTCUSDT","a":123,
		"p":"65000.10","q":"0.015","f":100,"l":101,"T":1700000000123,"m":false
	}`)
	c.handleAggTrade(raw)
	select {
	case ev := <-c.AggCh:
		if ev.TimeMs != 1700000000123 || ev.Price != 65000.10 || ev.Qty != 0.015 {
			t.Fatalf("%+v", ev)
		}
	default:
		t.Fatal("expected AggCh event")
	}
}

func TestHandleKlinePayload(t *testing.T) {
	c := NewWsClient("BTCUSDT", nil)
	raw := []byte(`{
		"e":"kline","E":1700000000000,"s":"BTCUSDT",
		"k":{"t":1700000000000,"T":1700000060000,"s":"BTCUSDT","i":"1m",
		"o":"65000","c":"65100","h":"65200","l":"64900","v":"12.3","V":"1.1","x":false}
	}`)
	c.handleKline(context.Background(), raw)
	select {
	case tick := <-c.OutCh:
		if tick.Timeframe != "1m" || tick.Kline.Close != 65100 || tick.Kline.Volume != 12.3 {
			t.Fatalf("unexpected tick: %+v", tick)
		}
	default:
		t.Fatal("expected OutCh")
	}
}

func TestHandleKlinePayloadNumericOHLC(t *testing.T) {
	raw := []byte(`{
		"e":"kline","E":1700000000000,"s":"BTCUSDT",
		"k":{"t":1700000000000,"T":1700000060000,"s":"BTCUSDT","i":"15m",
		"o":65000.1,"c":65100,"h":65200.5,"l":64900,"v":12.3,"V":1.1,"x":false}
	}`)
	tick, err := ParseFuturesWsKlineJSON(raw)
	if err != nil {
		t.Fatalf("parse numeric OHLC: %v", err)
	}
	if tick.Kline.Low != 64900 || tick.Kline.Close != 65100 || tick.Kline.Volume != 12.3 {
		t.Fatalf("got %+v", tick.Kline)
	}
}
