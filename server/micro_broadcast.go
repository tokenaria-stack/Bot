package server

import (
	"context"
	"time"
)

var defaultMicroTimeframes = []string{
	"1s", "5s", "10s", "15s", "30s", "45s",
	"1tick", "10ticks", "100ticks", "1000ticks",
}

const microBroadcastInterval = 500 * time.Millisecond

// StartMicroBroadcast launches the background micro-candle WebSocket broadcaster.
func (d *DashboardServer) StartMicroBroadcast(ctx context.Context) {
	go d.broadcastMicroTicks(ctx)
}

func (d *DashboardServer) broadcastMicroTicks(ctx context.Context) {
	ticker := time.NewTicker(microBroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.pushMicroTicks()
		}
	}
}

// pushMicroTicks sends the latest synthesized micro bar to subscribed dashboard clients.
func (d *DashboardServer) pushMicroTicks() {
	if d.orderFlow == nil || d.orderFlow.Ticks == nil {
		return
	}

	trades := d.orderFlow.Ticks.All()
	if len(trades) == 0 {
		return
	}

	latestByTF := make(map[string]ChartCandle, len(defaultMicroTimeframes))
	for _, tf := range defaultMicroTimeframes {
		if bar, ok := LatestMicroCandle(trades, tf); ok {
			latestByTF[tf] = bar
		}
	}

	d.clientsMu.Lock()
	type clientTick struct {
		client *WSClient
		bar    ChartCandle
		tf     string
	}
	pending := make([]clientTick, 0, len(d.clientTF))
	for client, tf := range d.clientTF {
		if !d.clients[client] {
			continue
		}
		spec, err := ResolveTimeframe(tf)
		if err != nil || !IsOrderFlowTimeframe(spec) {
			continue
		}
		bar, ok := latestByTF[spec.ID]
		if !ok {
			continue
		}
		pending = append(pending, clientTick{client: client, bar: bar, tf: spec.ID})
	}
	d.clientsMu.Unlock()

	var dead []*WSClient
	for _, item := range pending {
		if err := item.client.WriteJSON(wsEnvelope{
			Type: "tick",
			Data: tickPayload{
				Timeframe: item.tf,
				Time:      item.bar.Time,
				Open:      item.bar.Open,
				High:      item.bar.High,
				Low:       item.bar.Low,
				Close:     item.bar.Close,
			},
		}); err != nil {
			_ = item.client.Close()
			dead = append(dead, item.client)
		}
	}

	if len(dead) == 0 {
		return
	}

	d.clientsMu.Lock()
	for _, client := range dead {
		delete(d.clients, client)
		delete(d.clientTF, client)
	}
	d.clientsMu.Unlock()
}

func (d *DashboardServer) setClientTimeframe(client *WSClient, tf string) {
	spec, err := ResolveTimeframe(tf)
	if err != nil {
		return
	}
	d.clientsMu.Lock()
	d.clientTF[client] = spec.ID
	d.clientsMu.Unlock()
}
