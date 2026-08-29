package market

import (
	"context"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// DEBUG_SECONDS_HOT=1 logs a 10s rate snapshot of the always-warm seconds pipeline.
// Default off. Does not change fanout, persist, or transport.

var secondsHot struct {
	aggTrades      atomic.Uint64
	sec1Forming    atomic.Uint64
	sec1Closed     atomic.Uint64
	sparseForming  atomic.Uint64
	sparseClosed   atomic.Uint64
	onKlineBarCall atomic.Uint64
}

func secondsHotEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEBUG_SECONDS_HOT"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (m *Runtime) startSecondsHotAudit(ctx context.Context) {
	if m == nil || ctx == nil || !secondsHotEnabled() {
		return
	}
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				log.Printf("[SECONDS-HOT] per-10s aggTrade=%d 1sForming=%d 1sClosed=%d sparseForming=%d sparseClosed=%d klineBarCB=%d",
					secondsHot.aggTrades.Swap(0),
					secondsHot.sec1Forming.Swap(0),
					secondsHot.sec1Closed.Swap(0),
					secondsHot.sparseForming.Swap(0),
					secondsHot.sparseClosed.Swap(0),
					secondsHot.onKlineBarCall.Swap(0),
				)
			}
		}
	}()
}
