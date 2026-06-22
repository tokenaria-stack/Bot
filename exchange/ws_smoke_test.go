package exchange

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBinanceMarketCombinedStream(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	url := FuturesWSCombinedURL() + "btcusdt@aggTrade/btcusdt@kline_1m"
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if resp.StatusCode != 101 {
		t.Fatalf("status: %s", resp.Status)
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(msg) < 20 {
		t.Fatalf("short message: %q", msg)
	}
	t.Logf("got %d bytes", len(msg))
}
