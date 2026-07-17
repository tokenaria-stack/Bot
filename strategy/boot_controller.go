package strategy

// BootController (Core 5.0 Phase C) — explicit boot FSM killing the startup race
// where REST recovery ran before the WS connection existed, silently losing the
// bars that closed in between. Backend twin of the frontend pendingLiveTicks
// pattern: buffer live first, load history second, reconcile, then go live.
//
//	PhaseConnecting  — WS is up, ticks drain into a bounded buffer (no Marker writes)
//	PhaseLoading     — SQLite + REST recovery hydrate Markers (Ingress merge, Settled)
//	PhaseReconciling — buffered ticks replay through MasterGeneral.routeTick;
//	                   WS bars land after REST and finalize via the canonical
//	                   live path (AuthorityFinal semantics: WS never loses to REST)
//	PhaseLive        — direct WS → Marker flow (StartDataFeed), loops, strategies

import (
	"context"
	"log"
	"sync"

	"trading_bot/exchange"
)

// BootPhase is the explicit boot FSM state.
type BootPhase int32

const (
	PhaseConnecting BootPhase = iota
	PhaseLoading
	PhaseReconciling
	PhaseLive
)

func (p BootPhase) String() string {
	switch p {
	case PhaseConnecting:
		return "Connecting"
	case PhaseLoading:
		return "Loading"
	case PhaseReconciling:
		return "Reconciling"
	case PhaseLive:
		return "Live"
	default:
		return "Unknown"
	}
}

// defaultBootBufferCap bounds the boot tick buffer. Boot takes seconds;
// 10 TFs at a few kline updates/sec fit with a wide margin.
const defaultBootBufferCap = 4096

// BootController buffers live WS ticks while history loads, then replays them
// in arrival order through the canonical routing path.
type BootController struct {
	wsOut <-chan exchange.WsTick

	mu      sync.Mutex
	phase   BootPhase
	buffer  []exchange.WsTick
	dropped int

	stopBuffer chan struct{}
	bufferDone chan struct{}
}

// NewBootController wraps the WS output channel. Call Begin before the WS
// client starts pushing, then MarkLoading / Reconcile / GoLive in order.
func NewBootController(wsOut <-chan exchange.WsTick) *BootController {
	return &BootController{
		wsOut:      wsOut,
		buffer:     make([]exchange.WsTick, 0, 256),
		stopBuffer: make(chan struct{}),
		bufferDone: make(chan struct{}),
	}
}

// Begin enters PhaseConnecting: a goroutine drains wsOut into the buffer so
// the socket never backpressures while history loads. No Marker is touched.
func (b *BootController) Begin(ctx context.Context) {
	b.setPhase(PhaseConnecting)
	log.Println("[Boot] Phase 0: Connecting — WS ticks buffering, Markers untouched")
	go func() {
		defer close(b.bufferDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.stopBuffer:
				return
			case tick, ok := <-b.wsOut:
				if !ok {
					return
				}
				b.mu.Lock()
				if len(b.buffer) >= defaultBootBufferCap {
					// Keep newest: later ticks supersede older forming states.
					b.buffer = b.buffer[1:]
					b.dropped++
				}
				b.buffer = append(b.buffer, tick)
				b.mu.Unlock()
			}
		}
	}()
}

// MarkLoading enters PhaseLoading (history hydration runs in the caller).
func (b *BootController) MarkLoading() {
	b.setPhase(PhaseLoading)
	log.Println("[Boot] Phase 1: Loading — SQLite archive + REST recovery (WS buffering in background)")
}

// Reconcile stops buffering and replays every buffered tick, in arrival order,
// through MasterGeneral.routeTick — the same path live ticks take. Buffered WS
// bars therefore land after REST history and win on conflicts (Final > Settled).
func (b *BootController) Reconcile(m *MasterGeneral) int {
	b.setPhase(PhaseReconciling)
	close(b.stopBuffer)
	<-b.bufferDone

	b.mu.Lock()
	ticks := b.buffer
	b.buffer = nil
	dropped := b.dropped
	b.mu.Unlock()

	log.Printf("[Boot] Phase 2: Reconciling %d buffered ticks (dropped=%d)...", len(ticks), dropped)
	if m != nil {
		for _, tick := range ticks {
			m.routeTick(tick)
		}
	}
	return len(ticks)
}

// GoLive marks PhaseLive; the caller wires StartDataFeed on the same channel.
// Ticks arriving between Reconcile and StartDataFeed wait inside wsOut (cap 1000).
func (b *BootController) GoLive() {
	b.setPhase(PhaseLive)
	log.Println("[Boot] Phase 3: Live — direct WS → Marker flow")
}

// Phase returns the current boot phase (for logs/telemetry).
func (b *BootController) Phase() BootPhase {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.phase
}

func (b *BootController) setPhase(p BootPhase) {
	b.mu.Lock()
	b.phase = p
	b.mu.Unlock()
}
