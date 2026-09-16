package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"trading_bot/market"

	"github.com/gorilla/websocket"
)

func timelineStateTestServer(t *testing.T, master *market.Runtime) *httptest.Server {
	t.Helper()
	d := &DashboardServer{
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		clients:  make(map[*WSClient]bool),
		clientTF: make(map[*WSClient]string),
		master:   master,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", d.handleWS)
	return httptest.NewServer(mux)
}

func dialDashboardWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func readWSMap(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("json: %v body=%s", err, raw)
	}
	return msg
}

func TestTimelinePublishableNowNilMasterTrue(t *testing.T) {
	d := &DashboardServer{}
	if !d.timelinePublishableNow() {
		t.Fatal("nil Master must be publishable (no heal gate)")
	}
}

func TestTimelinePublishableNowEmptyRuntimeFalse(t *testing.T) {
	d := &DashboardServer{master: &market.Runtime{}}
	if d.timelinePublishableNow() {
		t.Fatal("zero Runtime publishable flag is false")
	}
}

func TestTimelineStateWelcomeTrue(t *testing.T) {
	srv := timelineStateTestServer(t, nil)
	defer srv.Close()
	conn := dialDashboardWS(t, srv)
	welcome := readWSMap(t, conn)
	if welcome["type"] != "welcome" {
		t.Fatalf("first=%v", welcome)
	}
	st := readWSMap(t, conn)
	if st["type"] != "timeline_state" {
		t.Fatalf("second=%v", st)
	}
	if st["publishable"] != true {
		t.Fatalf("publishable=%v", st["publishable"])
	}
}

func TestTimelineStateWelcomeFalse(t *testing.T) {
	srv := timelineStateTestServer(t, &market.Runtime{})
	defer srv.Close()
	conn := dialDashboardWS(t, srv)
	_ = readWSMap(t, conn) // welcome
	st := readWSMap(t, conn)
	if st["type"] != "timeline_state" {
		t.Fatalf("got %v", st)
	}
	if st["publishable"] != false {
		t.Fatalf("publishable=%v", st["publishable"])
	}
}

func TestTimelineStateRequestReply(t *testing.T) {
	srv := timelineStateTestServer(t, nil)
	defer srv.Close()
	conn := dialDashboardWS(t, srv)
	_ = readWSMap(t, conn)
	_ = readWSMap(t, conn)
	if err := conn.WriteJSON(map[string]string{"type": "timeline_state_request"}); err != nil {
		t.Fatal(err)
	}
	st := readWSMap(t, conn)
	if st["type"] != "timeline_state" || st["publishable"] != true {
		t.Fatalf("reply=%v", st)
	}
}

func TestSubscribeDoesNotEmitTimelineState(t *testing.T) {
	srv := timelineStateTestServer(t, nil)
	defer srv.Close()
	conn := dialDashboardWS(t, srv)
	_ = readWSMap(t, conn)
	_ = readWSMap(t, conn)
	if err := conn.WriteJSON(map[string]any{"type": "subscribe", "tf": "5m"}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	_, raw, err := conn.ReadMessage()
	if err == nil {
		t.Fatalf("unexpected message after subscribe: %s", raw)
	}
}

func TestTimelineStatePayloadShape(t *testing.T) {
	p := timelineStatePayload(true)
	if p["type"] != "timeline_state" || p["publishable"] != true {
		t.Fatalf("%v", p)
	}
	p = timelineStatePayload(false)
	if p["publishable"] != false {
		t.Fatalf("%v", p)
	}
}
